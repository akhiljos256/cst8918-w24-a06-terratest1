package test

import (
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/azure"
	"github.com/gruntwork-io/terratest/modules/shell"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/assert"
)

// You normally want to run this under a separate "Testing" subscription
// For lab purposes you will use your assigned subscription under the Cloud Dev/Ops program tenant
var subscriptionID string = "5eb83737-e0c8-46c1-818d-4d9725820e3f"

func cleanupAzureResources(t *testing.T, terraformOptions *terraform.Options) {
	// Get resource names from outputs
	resourceGroupName := terraform.Output(t, terraformOptions, "resource_group_name")
	vmName := terraform.Output(t, terraformOptions, "vm_name")

	// Delete VM and its disk first using az CLI if it exists
	if azure.VirtualMachineExists(t, vmName, resourceGroupName, subscriptionID) {
		// Delete VM
		cmd := shell.Command{
			Command: "az",
			Args: []string{
				"vm",
				"delete",
				"--resource-group", resourceGroupName,
				"--name", vmName,
				"--yes",
			},
		}
		shell.RunCommand(t, cmd)

		// Delete OS disk
		diskName := vmName + "OSDisk"
		cmd = shell.Command{
			Command: "az",
			Args: []string{
				"disk",
				"delete",
				"--resource-group", resourceGroupName,
				"--name", diskName,
				"--yes",
			},
		}
		shell.RunCommand(t, cmd)

		time.Sleep(30 * time.Second) // Wait for deletions to complete
	}

	// Now run terraform destroy
	terraform.Destroy(t, terraformOptions)
}

func TestAzureLinuxVMCreation(t *testing.T) {
	terraformOptions := &terraform.Options{
		// The path to where our Terraform code is located
		TerraformDir: "../",
		// Override the default terraform variables
		Vars: map[string]interface{}{
			"labelPrefix": "jose0337",
		},
		// Add retry options for more graceful cleanup
		MaxRetries:         5,
		TimeBetweenRetries: 10 * time.Second,
		RetryableTerraformErrors: map[string]string{
			"InternalServerError":                  "Internal Server Error",
			"NetworkSecurityGroupOldReferencesNotCleanedUp": "Network security group cannot be deleted",
		},
	}

	// Make sure to clean up resources even if the test fails
	defer cleanupAzureResources(t, terraformOptions)

	// Run `terraform init` and `terraform apply`
	terraform.InitAndApply(t, terraformOptions)

	// Run `terraform output` to get the value of output variable
	vmName := terraform.Output(t, terraformOptions, "vm_name")
	resourceGroupName := terraform.Output(t, terraformOptions, "resource_group_name")

	// Confirm VM exists
	assert.True(t, azure.VirtualMachineExists(t, vmName, resourceGroupName, subscriptionID))

	// Test 2: Confirm NIC exists and is connected to VM
	assert.True(t, azure.NetworkInterfaceExists(t, nicName, resourceGroupName, subscriptionID), "NIC does not exist")
	nic := azure.GetNetworkInterface(t, nicName, resourceGroupName, subscriptionID)
	assert.Equal(t, vmName, *nic.VirtualMachine.ID, "NIC is not attached to the correct VM")

	// Test 3: Confirm the VM is running the correct Ubuntu version
	expectedUbuntuVersion := "22.04" // Change this based on the Terraform configuration
	vm := azure.GetVirtualMachine(t, vmName, resourceGroupName, subscriptionID)
	assert.Contains(t, *vm.StorageProfile.ImageReference.Offer, "Ubuntu", "VM is not running Ubuntu")
	assert.Contains(t, *vm.StorageProfile.ImageReference.Sku, expectedUbuntuVersion, "VM is not running the expected Ubuntu version")
}
