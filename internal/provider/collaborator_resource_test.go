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

func TestAccCollaboratorResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name  = "test_repo"
}
resource "forgejo_user" "test" {
	login    = "tftest"
	email    = "tftest@localhost.localdomain"
	password = "passw0rd"
}
resource "forgejo_collaborator" "test" {
	repository_id = forgejo_repository.test.id
	user          = forgejo_user.test.login
	permission    = "read"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_collaborator.test", tfjsonpath.New("repository_id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_collaborator.test", tfjsonpath.New("user"), knownvalue.StringExact("tftest")),
					statecheck.ExpectKnownValue("forgejo_collaborator.test", tfjsonpath.New("permission"), knownvalue.StringExact("read")),
				},
			},
			// Update and Read testing
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name  = "test_repo"
}
resource "forgejo_user" "test" {
	login    = "tftest"
	email    = "tftest@localhost.localdomain"
	password = "passw0rd"
}
resource "forgejo_collaborator" "test" {
	repository_id = forgejo_repository.test.id
	user          = forgejo_user.test.login
	permission    = "write"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_collaborator.test", tfjsonpath.New("repository_id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_collaborator.test", tfjsonpath.New("user"), knownvalue.StringExact("tftest")),
					statecheck.ExpectKnownValue("forgejo_collaborator.test", tfjsonpath.New("permission"), knownvalue.StringExact("write")),
				},
			},
			// Update and Read testing
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name  = "test_repo"
}
resource "forgejo_user" "test" {
	login    = "tftest"
	email    = "tftest@localhost.localdomain"
	password = "passw0rd"
}
resource "forgejo_collaborator" "test" {
	repository_id = forgejo_repository.test.id
	user          = forgejo_user.test.login
	permission    = "admin"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_collaborator.test", tfjsonpath.New("repository_id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_collaborator.test", tfjsonpath.New("user"), knownvalue.StringExact("tftest")),
					statecheck.ExpectKnownValue("forgejo_collaborator.test", tfjsonpath.New("permission"), knownvalue.StringExact("admin")),
				},
			},
			// Import testing
			{
				ResourceName: "forgejo_collaborator.test",
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					repoRS := s.RootModule().Resources["forgejo_repository.test"]
					collabRS := s.RootModule().Resources["forgejo_collaborator.test"]
					owner := repoRS.Primary.Attributes["owner"]
					repoName := repoRS.Primary.Attributes["name"]
					username := collabRS.Primary.Attributes["user"]
					return fmt.Sprintf("%s:%s:%s", owner, repoName, username), nil
				},
				ImportStateCheck: func(s []*terraform.InstanceState) error {
					if len(s) != 1 {
						return fmt.Errorf("expected 1 state, got %d", len(s))
					}
					if _, ok := s[0].Attributes["repository_id"]; !ok {
						return fmt.Errorf("repository_id not found in state")
					}
					if _, ok := s[0].Attributes["user"]; !ok {
						return fmt.Errorf("user not found in state")
					}
					if _, ok := s[0].Attributes["permission"]; !ok {
						return fmt.Errorf("permission not found in state")
					}
					return nil
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}
