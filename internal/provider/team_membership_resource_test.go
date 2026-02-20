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

func TestAccTeamMembershipResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_membership_org"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "developers"
	is_admin     = true
}

resource "forgejo_user" "test" {
	login    = "tftest_member"
	email    = "tftest_member@localhost.localdomain"
	password = "passw0rd"
}

resource "forgejo_team_membership" "test" {
	team_id  = forgejo_team.test.id
	username = forgejo_user.test.login
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_team_membership.test", tfjsonpath.New("team_id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_team_membership.test", tfjsonpath.New("username"), knownvalue.StringExact("tftest_member")),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccTeamMembershipResource_Import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create a team membership
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_membership_import"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "importable"
	is_admin     = true
}

resource "forgejo_user" "test" {
	login    = "tftest_import"
	email    = "tftest_import@localhost.localdomain"
	password = "passw0rd"
}

resource "forgejo_team_membership" "test" {
	team_id  = forgejo_team.test.id
	username = forgejo_user.test.login
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_team_membership.test", tfjsonpath.New("team_id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_team_membership.test", tfjsonpath.New("username"), knownvalue.StringExact("tftest_import")),
				},
			},
			// Import the team membership using custom ID format: organization:team_name:username
			{
				ResourceName: "forgejo_team_membership.test",
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					// Get the team membership data from the state
					teamRS := s.RootModule().Resources["forgejo_team.test"]
					memberRS := s.RootModule().Resources["forgejo_team_membership.test"]
					org := teamRS.Primary.Attributes["organization"]
					teamName := teamRS.Primary.Attributes["name"]
					username := memberRS.Primary.Attributes["username"]
					return fmt.Sprintf("%s:%s:%s", org, teamName, username), nil
				},
				ImportStateCheck: func(s []*terraform.InstanceState) error {
					// Verify that both fields are present after import
					if len(s) != 1 {
						return fmt.Errorf("expected 1 state, got %d", len(s))
					}
					if _, ok := s[0].Attributes["team_id"]; !ok {
						return fmt.Errorf("team_id not found in state")
					}
					if _, ok := s[0].Attributes["username"]; !ok {
						return fmt.Errorf("username not found in state")
					}
					return nil
				},
			},
		},
	})
}
