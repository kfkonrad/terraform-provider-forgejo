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

func TestAccRepositoryLabelResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing - basic label
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name      = "test_label_repo"
	auto_init = true
}
resource "forgejo_repository_label" "test" {
	repository  = forgejo_repository.test.full_name
	name        = "bug"
	color       = "d73a4a"
	description = "Something isn't working"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_label.test", tfjsonpath.New("repository"), knownvalue.StringExact("tfadmin/test_label_repo")),
					statecheck.ExpectKnownValue("forgejo_repository_label.test", tfjsonpath.New("owner"), knownvalue.StringExact("tfadmin")),
					statecheck.ExpectKnownValue("forgejo_repository_label.test", tfjsonpath.New("name"), knownvalue.StringExact("bug")),
					statecheck.ExpectKnownValue("forgejo_repository_label.test", tfjsonpath.New("color"), knownvalue.StringExact("d73a4a")),
					statecheck.ExpectKnownValue("forgejo_repository_label.test", tfjsonpath.New("description"), knownvalue.StringExact("Something isn't working")),
					statecheck.ExpectKnownValue("forgejo_repository_label.test", tfjsonpath.New("id"), knownvalue.NotNull()),
				},
			},
			// Import testing
			{
				ResourceName: "forgejo_repository_label.test",
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["forgejo_repository_label.test"]
					owner := rs.Primary.Attributes["owner"]
					id := rs.Primary.Attributes["id"]

					return fmt.Sprintf("%s:%s:%s", owner, "test_label_repo", id), nil
				},
				ImportStateVerify: true,
			},
			// Update testing - change color and description
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name      = "test_label_repo"
	auto_init = true
}
resource "forgejo_repository_label" "test" {
	repository  = forgejo_repository.test.full_name
	name        = "bug"
	color       = "cc0000"
	description = "Confirmed bug"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_label.test", tfjsonpath.New("color"), knownvalue.StringExact("cc0000")),
					statecheck.ExpectKnownValue("forgejo_repository_label.test", tfjsonpath.New("description"), knownvalue.StringExact("Confirmed bug")),
				},
			},
		},
	})
}

func TestAccRepositoryLabelResource_OrganizationRepository(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "test-label-org"
}
resource "forgejo_repository" "test" {
	owner     = forgejo_organization.test.name
	name      = "test_repo"
	auto_init = true
}
resource "forgejo_repository_label" "test" {
	repository = forgejo_repository.test.full_name
	name       = "enhancement"
	color      = "a2eeef"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_label.test", tfjsonpath.New("repository"), knownvalue.StringExact("test-label-org/test_repo")),
					statecheck.ExpectKnownValue("forgejo_repository_label.test", tfjsonpath.New("owner"), knownvalue.StringExact("test-label-org")),
					statecheck.ExpectKnownValue("forgejo_repository_label.test", tfjsonpath.New("name"), knownvalue.StringExact("enhancement")),
				},
			},
		},
	})
}
