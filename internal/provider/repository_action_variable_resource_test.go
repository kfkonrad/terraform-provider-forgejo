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

func TestAccRepositoryActionVariableResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name = "test_action_var_repo"
}
resource "forgejo_repository_action_variable" "test" {
	repository_id = forgejo_repository.test.id
	name          = "MY_VARIABLE"
	value         = "my_variable_value"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_action_variable.test", tfjsonpath.New("repository_id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_repository_action_variable.test", tfjsonpath.New("name"), knownvalue.StringExact("MY_VARIABLE")),
					statecheck.ExpectKnownValue("forgejo_repository_action_variable.test", tfjsonpath.New("value"), knownvalue.StringExact("my_variable_value")),
				},
			},
			// ImportState testing
			{
				ImportState:  true,
				ResourceName: "forgejo_repository_action_variable.test",
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["forgejo_repository_action_variable.test"]
					repoID := rs.Primary.Attributes["repository_id"]
					name := rs.Primary.Attributes["name"]

					return fmt.Sprintf("%s:%s", repoID, name), nil
				},
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
			// Update and Read testing
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name = "test_action_var_repo"
}
resource "forgejo_repository_action_variable" "test" {
	repository_id = forgejo_repository.test.id
	name          = "MY_VARIABLE"
	value         = "my_new_variable_value"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_action_variable.test", tfjsonpath.New("repository_id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_repository_action_variable.test", tfjsonpath.New("name"), knownvalue.StringExact("MY_VARIABLE")),
					statecheck.ExpectKnownValue("forgejo_repository_action_variable.test", tfjsonpath.New("value"), knownvalue.StringExact("my_new_variable_value")),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}
