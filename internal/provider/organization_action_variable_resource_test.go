package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccOrganizationActionVariableResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "test_action_var_org"
}
resource "forgejo_organization_action_variable" "test" {
	organization = forgejo_organization.test.name
	name         = "my_variable"
	value        = "my_variable_value"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_organization_action_variable.test", tfjsonpath.New("organization"), knownvalue.StringExact("test_action_var_org")),
					statecheck.ExpectKnownValue("forgejo_organization_action_variable.test", tfjsonpath.New("name"), knownvalue.StringExact("my_variable")),
					statecheck.ExpectKnownValue("forgejo_organization_action_variable.test", tfjsonpath.New("value"), knownvalue.StringExact("my_variable_value")),
				},
			},
			// ImportState testing
			{
				ImportState:  true,
				ResourceName: "forgejo_organization_action_variable.test",
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["forgejo_organization_action_variable.test"]
					org := rs.Primary.Attributes["organization"]

					return fmt.Sprintf("%s:%s", org, "my_variable"), nil
				},
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "test_action_var_org"
}
resource "forgejo_organization_action_variable" "test" {
	organization = forgejo_organization.test.name
	name         = "my_variable"
	value        = "my_new_variable_value"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_organization_action_variable.test", tfjsonpath.New("organization"), knownvalue.StringExact("test_action_var_org")),
					statecheck.ExpectKnownValue("forgejo_organization_action_variable.test", tfjsonpath.New("name"), knownvalue.StringExact("my_variable")),
					statecheck.ExpectKnownValue("forgejo_organization_action_variable.test", tfjsonpath.New("value"), knownvalue.StringExact("my_new_variable_value")),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}
