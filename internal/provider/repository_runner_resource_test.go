package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccRepositoryRunnerResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name      = "test_runner_repo"
	auto_init = true
}
resource "forgejo_repository_runner" "test" {
	repository = forgejo_repository.test.full_name
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_runner.test", tfjsonpath.New("repository"), knownvalue.StringExact("tfadmin/test_runner_repo")),
					statecheck.ExpectKnownValue("forgejo_repository_runner.test", tfjsonpath.New("owner"), knownvalue.StringExact("tfadmin")),
					statecheck.ExpectKnownValue("forgejo_repository_runner.test", tfjsonpath.New("token"), knownvalue.NotNull()),
				},
			},
		},
	})
}
