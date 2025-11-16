package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccTeamResource_Granular(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing with granular_permissions block
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_org"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "developers"
	permission   = "granular"
	granular_permissions {
		code   = "write"
		issues = "read"
	}
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("organization"), knownvalue.StringExact("tftest_team_org")),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("name"), knownvalue.StringExact("developers")),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("permission"), knownvalue.StringExact("granular")),
				},
			},
			// Update and Read testing - change granular permissions
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
	permission                = "granular"
	granular_permissions {
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
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("permission"), knownvalue.StringExact("granular")),
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
	permission                = "granular"
	granular_permissions {
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
	permission   = "admin"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("permission"), knownvalue.StringExact("admin")),
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
	permission   = "admin"
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
			// Create with granular permission but minimal access block - all units default to "none"
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_minimal"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "readers"
	permission   = "granular"
	granular_permissions {
		code = "read"
	}
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("permission"), knownvalue.StringExact("granular")),
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
	permission   = "granular"
	granular_permissions {
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

func TestAccTeamResource_AdminConflict(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Try to create team with both admin permission and granular_permissions - should error
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_conflict"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "invalid"
	permission   = "admin"
	granular_permissions {
		code = "write"
	}
}
`,
				ExpectError: regexp.MustCompile("Invalid permission configuration"),
			},
		},
	})
}

func TestAccTeamResource_GranularRequired(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Try to create team with granular permission but no granular_permissions block - should error
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_required"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "invalid"
	permission   = "granular"
}
`,
				ExpectError: regexp.MustCompile("Invalid permission configuration"),
			},
		},
	})
}
