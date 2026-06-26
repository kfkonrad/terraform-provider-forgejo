package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccOrganizationRunnerResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "test-runner-org"
}
resource "forgejo_organization_runner" "test" {
	organization = forgejo_organization.test.name
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_organization_runner.test", tfjsonpath.New("organization"), knownvalue.StringExact("test-runner-org")),
					statecheck.ExpectKnownValue("forgejo_organization_runner.test", tfjsonpath.New("token"), knownvalue.NotNull()),
				},
			},
		},
	})
}
