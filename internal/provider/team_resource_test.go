package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccTeamResource_Granular(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing with permissions block
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_org"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "developers"
	is_admin     = false
	permissions {
		code   = "write"
		issues = "read"
	}
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("organization"), knownvalue.StringExact("tftest_team_org")),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("name"), knownvalue.StringExact("developers")),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("is_admin"), knownvalue.Bool(false)),
				},
			},
			// Update and Read testing - change permissions
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_org"
}

resource "forgejo_team" "test" {
	organization              = forgejo_organization.test.name
	name                      = "developers"
	description               = "Development team"
	can_create_org_repo       = true
	includes_all_repositories = true
	is_admin                  = false
	permissions {
		code   = "write"
		issues = "write"
		pulls  = "write"
	}
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("organization"), knownvalue.StringExact("tftest_team_org")),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("name"), knownvalue.StringExact("developers")),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("description"), knownvalue.StringExact("Development team")),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("can_create_org_repo"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("includes_all_repositories"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("is_admin"), knownvalue.Bool(false)),
				},
			},
			// Re-plan to ensure no drift
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_org"
}

resource "forgejo_team" "test" {
	organization              = forgejo_organization.test.name
	name                      = "developers"
	description               = "Development team"
	can_create_org_repo       = true
	includes_all_repositories = true
	is_admin                  = false
	permissions {
		code   = "write"
		issues = "write"
		pulls  = "write"
	}
}
`,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false, // Should not plan any changes
			},
		},
	})
}

func TestAccTeamResource_Admin(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create team with admin permission
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_admin"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "admins"
	is_admin     = true
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("is_admin"), knownvalue.Bool(true)),
				},
			},
			// Re-plan to ensure no drift
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_admin"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "admins"
	is_admin     = true
}
`,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false, // Should not plan any changes
			},
		},
	})
}

func TestAccTeamResource_GranularMinimal(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with is_admin=false but minimal permissions block
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_minimal"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "readers"
	is_admin     = false
	permissions {
		code = "read"
	}
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("is_admin"), knownvalue.Bool(false)),
				},
			},
			// Re-plan to ensure no drift
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_minimal"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "readers"
	is_admin     = false
	permissions {
		code = "read"
	}
}
`,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false, // Should not plan any changes
			},
		},
	})
}

func TestAccTeamResource_AdminIgnoresPermissions(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create team with is_admin=true and permissions block - should work, permissions ignored
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_ignore"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "managers"
	is_admin     = true
	permissions {
		code = "write"
	}
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("is_admin"), knownvalue.Bool(true)),
				},
			},
			// Re-plan to ensure no drift (permissions block should be ignored)
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_ignore"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "managers"
	is_admin     = true
	permissions {
		code = "write"
	}
}
`,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false, // Should not plan any changes
			},
		},
	})
}

func TestAccTeamResource_GranularRequired(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Try to create team with is_admin=false but no permissions block - should error
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_required"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "invalid"
	is_admin     = false
}
`,
				ExpectError: regexp.MustCompile("Invalid permission configuration"),
			},
		},
	})
}

func TestAccTeamResource_Import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create a team
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_import"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "importable"
	is_admin     = true
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("name"), knownvalue.StringExact("importable")),
				},
			},
			// Import the team using custom ID format: organization/team_id
			{
				ResourceName:      "forgejo_team.test",
				ImportState:       true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					// Get the team ID from the state
					rs := s.RootModule().Resources["forgejo_team.test"]
					id := rs.Primary.Attributes["id"]
					org := rs.Primary.Attributes["organization"]
					return fmt.Sprintf("%s/%s", org, id), nil
				},
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"description", "can_create_org_repo", "includes_all_repositories"}, // These fields are computed/default
			},
		},
	})
}
