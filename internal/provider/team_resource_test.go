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
			// Create and Read testing with permissions block
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_org"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "developers"
	permissions {
		level = "write"
		units = ["repo.code", "repo.issues"]
	}
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("organization"), knownvalue.StringExact("tftest_team_org")),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("name"), knownvalue.StringExact("developers")),
				},
			},
			// Update and Read testing - change permissions level
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
	permissions {
		level = "admin"
		units = ["repo.code", "repo.issues", "repo.pulls"]
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
	permissions {
		level = "admin"
		units = ["repo.code", "repo.issues", "repo.pulls"]
	}
}
`,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false, // Should not plan any changes
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccTeamResource_WithUnitsMap(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with units_map
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_unitsmap"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "developers"
	description  = "Development team"
	units_map = {
		"repo.code"   = "read"
		"repo.issues" = "write"
		"repo.pulls"  = "read"
	}
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("organization"), knownvalue.StringExact("tftest_team_unitsmap")),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("name"), knownvalue.StringExact("developers")),
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("description"), knownvalue.StringExact("Development team")),
				},
			},
			// Re-plan to ensure no drift
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_unitsmap"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "developers"
	description  = "Development team"
	units_map = {
		"repo.code"   = "read"
		"repo.issues" = "write"
		"repo.pulls"  = "read"
	}
}
`,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false, // Should not plan any changes
			},
			// Update units_map
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_unitsmap"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "developers"
	description  = "Development team"
	units_map = {
		"repo.code"   = "write"
		"repo.issues" = "write"
		"repo.pulls"  = "write"
	}
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("id"), knownvalue.NotNull()),
				},
			},
			// Re-plan after units_map update to ensure no drift
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_unitsmap"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "developers"
	description  = "Development team"
	units_map = {
		"repo.code"   = "write"
		"repo.issues" = "write"
		"repo.pulls"  = "write"
	}
}
`,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false, // Should not plan any changes
			},
		},
	})
}

func TestAccTeamResource_SwitchFromPermissionsToUnitsMap(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with permissions block
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_switch1"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "developers"
	permissions {
		level = "write"
		units = ["repo.code", "repo.issues"]
	}
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("id"), knownvalue.NotNull()),
				},
			},
			// Switch to units_map (resource will be replaced due to mutually exclusive attrs)
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_switch1"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "developers"
	units_map = {
		"repo.code"   = "write"
		"repo.issues" = "write"
	}
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("id"), knownvalue.NotNull()),
				},
			},
		},
	})
}

func TestAccTeamResource_SwitchFromUnitsMapToPermissions(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with units_map
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_switch2"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "developers"
	units_map = {
		"repo.code"   = "read"
		"repo.issues" = "write"
	}
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("id"), knownvalue.NotNull()),
				},
			},
			// Switch to permissions block (resource will be replaced due to mutually exclusive attrs)
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "tftest_team_switch2"
}

resource "forgejo_team" "test" {
	organization = forgejo_organization.test.name
	name         = "developers"
	permissions {
		level = "write"
		units = ["repo.code", "repo.issues"]
	}
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_team.test", tfjsonpath.New("id"), knownvalue.NotNull()),
				},
			},
		},
	})
}
