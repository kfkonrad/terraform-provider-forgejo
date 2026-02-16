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

func TestAccRepositoryBranchRuleResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing - basic branch rule
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name      = "test_branch_rule_repo"
	auto_init = true
}
resource "forgejo_repository_branch_rule" "test" {
	repository  = forgejo_repository.test.full_name
	protected_branch_pattern   = "main"
	enable_push = true
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("repository"), knownvalue.StringExact("tfadmin/test_branch_rule_repo")),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("owner"), knownvalue.StringExact("tfadmin")),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("protected_branch_pattern"), knownvalue.StringExact("main")),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("enable_push"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("enable_push_whitelist"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("require_signed_commits"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("required_approvals"), knownvalue.Int64Exact(0)),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("enable_status_check"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("enable_merge_whitelist"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("block_on_rejected_reviews"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("block_on_official_review_requests"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("block_on_outdated_branch"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("protected_file_patterns"), knownvalue.StringExact("")),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("unprotected_file_patterns"), knownvalue.StringExact("")),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("created_at"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("updated_at"), knownvalue.NotNull()),
				},
			},
			// Update testing - enable more protections
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name      = "test_branch_rule_repo"
	auto_init = true
}
resource "forgejo_repository_branch_rule" "test" {
	repository                     = forgejo_repository.test.full_name
	protected_branch_pattern                      = "main"
	enable_push                    = true
	required_approvals             = 2
	dismiss_stale_approvals        = true
	block_on_rejected_reviews      = true
	block_on_outdated_branch       = true
	protected_file_patterns        = "LICENSE;*.lock"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("enable_push"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("required_approvals"), knownvalue.Int64Exact(2)),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("dismiss_stale_approvals"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("block_on_rejected_reviews"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("block_on_outdated_branch"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("protected_file_patterns"), knownvalue.StringExact("LICENSE;*.lock")),
				},
			},
			// Import testing
			{
				ResourceName: "forgejo_repository_branch_rule.test",
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["forgejo_repository_branch_rule.test"]
					owner := rs.Primary.Attributes["owner"]
					pattern := rs.Primary.Attributes["protected_branch_pattern"]

					return fmt.Sprintf("%s:%s:%s", owner, "test_branch_rule_repo", pattern), nil
				},
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "protected_branch_pattern",
			},
		},
	})
}

func TestAccRepositoryBranchRuleResource_OrganizationRepository(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing - organization repository
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "test-branch-rule-org"
}
resource "forgejo_repository" "test" {
	owner     = forgejo_organization.test.name
	name      = "test_repo"
	auto_init = true
}
resource "forgejo_repository_branch_rule" "test" {
	repository                = forgejo_repository.test.full_name
	protected_branch_pattern                 = "main"
	enable_push               = true
	require_signed_commits    = true
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("repository"), knownvalue.StringExact("test-branch-rule-org/test_repo")),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("owner"), knownvalue.StringExact("test-branch-rule-org")),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("protected_branch_pattern"), knownvalue.StringExact("main")),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("enable_push"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("require_signed_commits"), knownvalue.Bool(true)),
				},
			},
			// Import testing for organization repository
			{
				ResourceName: "forgejo_repository_branch_rule.test",
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["forgejo_repository_branch_rule.test"]
					owner := rs.Primary.Attributes["owner"]
					pattern := rs.Primary.Attributes["protected_branch_pattern"]

					return fmt.Sprintf("%s:%s:%s", owner, "test_repo", pattern), nil
				},
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "protected_branch_pattern",
			},
		},
	})
}

func TestAccRepositoryBranchRuleResource_GlobPattern(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with glob pattern
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name      = "test_glob_branch_rule_repo"
	auto_init = true
}
resource "forgejo_repository_branch_rule" "test" {
	repository  = forgejo_repository.test.full_name
	protected_branch_pattern   = "release/**"
	enable_push = true
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("protected_branch_pattern"), knownvalue.StringExact("release/**")),
					statecheck.ExpectKnownValue("forgejo_repository_branch_rule.test", tfjsonpath.New("enable_push"), knownvalue.Bool(true)),
				},
			},
		},
	})
}
