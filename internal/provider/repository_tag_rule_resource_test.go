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

func TestAccRepositoryTagRuleResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing - basic tag rule
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name      = "test_tag_rule_repo"
	auto_init = true
}
resource "forgejo_repository_tag_rule" "test" {
	repository            = forgejo_repository.test.full_name
	protected_tag_pattern = "v*"
	whitelist_usernames   = ["tfadmin"]
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_tag_rule.test", tfjsonpath.New("repository"), knownvalue.StringExact("tfadmin/test_tag_rule_repo")),
					statecheck.ExpectKnownValue("forgejo_repository_tag_rule.test", tfjsonpath.New("owner"), knownvalue.StringExact("tfadmin")),
					statecheck.ExpectKnownValue("forgejo_repository_tag_rule.test", tfjsonpath.New("protected_tag_pattern"), knownvalue.StringExact("v*")),
					statecheck.ExpectKnownValue("forgejo_repository_tag_rule.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_repository_tag_rule.test", tfjsonpath.New("whitelist_usernames"), knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact("tfadmin"),
					})),
					statecheck.ExpectKnownValue("forgejo_repository_tag_rule.test", tfjsonpath.New("created_at"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_repository_tag_rule.test", tfjsonpath.New("updated_at"), knownvalue.NotNull()),
				},
			},
			// Import testing
			{
				ResourceName: "forgejo_repository_tag_rule.test",
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["forgejo_repository_tag_rule.test"]
					owner := rs.Primary.Attributes["owner"]
					id := rs.Primary.Attributes["id"]

					return fmt.Sprintf("%s:%s:%s", owner, "test_tag_rule_repo", id), nil
				},
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccRepositoryTagRuleResource_OrganizationRepository(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing - organization repository
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "test-tag-rule-org"
}
resource "forgejo_repository" "test" {
	owner     = forgejo_organization.test.name
	name      = "test_repo"
	auto_init = true
}
resource "forgejo_repository_tag_rule" "test" {
	repository            = forgejo_repository.test.full_name
	protected_tag_pattern = "v*"
	whitelist_usernames   = ["tfadmin"]
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_tag_rule.test", tfjsonpath.New("repository"), knownvalue.StringExact("test-tag-rule-org/test_repo")),
					statecheck.ExpectKnownValue("forgejo_repository_tag_rule.test", tfjsonpath.New("owner"), knownvalue.StringExact("test-tag-rule-org")),
					statecheck.ExpectKnownValue("forgejo_repository_tag_rule.test", tfjsonpath.New("protected_tag_pattern"), knownvalue.StringExact("v*")),
				},
			},
			// Import testing for organization repository
			{
				ResourceName: "forgejo_repository_tag_rule.test",
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["forgejo_repository_tag_rule.test"]
					owner := rs.Primary.Attributes["owner"]
					id := rs.Primary.Attributes["id"]

					return fmt.Sprintf("%s:%s:%s", owner, "test_repo", id), nil
				},
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccRepositoryTagRuleResource_GlobPattern(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with glob pattern
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name      = "test_glob_tag_rule_repo"
	auto_init = true
}
resource "forgejo_repository_tag_rule" "test" {
	repository            = forgejo_repository.test.full_name
	protected_tag_pattern = "release/**"
	whitelist_usernames   = ["tfadmin"]
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_tag_rule.test", tfjsonpath.New("protected_tag_pattern"), knownvalue.StringExact("release/**")),
				},
			},
		},
	})
}
