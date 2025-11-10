package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccTeamResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing with access block
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_org"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "developers"
	permission   = "write"
	access {
		code   = "write"
		issues = "read"
	}
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("organization"), knownvalue.StringExact("tftest_team_org")),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("name"), knownvalue.StringExact("developers")),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("permission"), knownvalue.StringExact("write")),
				},
			},
			// Update and Read testing - change permission level
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
	permission                = "admin"
	access {
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
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("permission"), knownvalue.StringExact("admin")),
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
	permission                = "admin"
	access {
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

func TestAccTeamResource_EmptyAccess(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with permission but no access block - all units should default to "none"
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_noaccess"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "readers"
	permission   = "read"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("permission"), knownvalue.StringExact("read")),
				},
			},
			// Re-plan to ensure no drift
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_noaccess"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "readers"
	permission   = "read"
}
`,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false, // Should not plan any changes
			},
		},
	})
}

func TestAccTeamResource_PartialAccess(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with some access keys specified, others default to "none"
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_partial"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "managers"
	permission   = "admin"
	access {
		code    = "admin"
		issues  = "admin"
		wiki    = "write"
	}
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
	name = "tftest_team_partial"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "managers"
	permission   = "admin"
	access {
		code    = "admin"
		issues  = "admin"
		wiki    = "write"
	}
}
`,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false, // Should not plan any changes
			},
		},
	})
}
