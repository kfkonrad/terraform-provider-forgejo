package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAccessTokenResource_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckAccessToken(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: providerConfig + `
resource "forgejo_user" "test" {
	login    = "token_test_user"
	email    = "token_test@localhost.localdomain"
	password = "passw0rd"
}

resource "forgejo_access_token" "test" {
	username = forgejo_user.test.login
	name     = "test-token"
	scopes   = ["read:repository"]
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("username"), knownvalue.StringExact("token_test_user")),
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("name"), knownvalue.StringExact("test-token")),
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("token"), knownvalue.NotNull()),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccAccessTokenResource_WithScopes(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckAccessToken(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing with single scope
			{
				Config: providerConfig + `
resource "forgejo_user" "test" {
	login    = "token_scopes_user"
	email    = "token_scopes@localhost.localdomain"
	password = "passw0rd"
}

resource "forgejo_access_token" "test" {
	username = forgejo_user.test.login
	name     = "test-token-scopes"
	scopes   = ["write:repository"]
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("username"), knownvalue.StringExact("token_scopes_user")),
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("name"), knownvalue.StringExact("test-token-scopes")),
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("scopes"), knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact("write:repository"),
					})),
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("token"), knownvalue.NotNull()),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccAccessTokenResource_ImmutableName(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckAccessToken(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create token
			{
				Config: providerConfig + `
resource "forgejo_user" "test" {
	login    = "token_immutable_user"
	email    = "token_immutable@localhost.localdomain"
	password = "passw0rd"
}

resource "forgejo_access_token" "test" {
	username = forgejo_user.test.login
	name     = "immutable-test"
	scopes   = ["read:repository"]
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("name"), knownvalue.StringExact("immutable-test")),
				},
			},
			// Try to update name - should force replacement
			{
				Config: providerConfig + `
resource "forgejo_user" "test" {
	login    = "token_immutable_user"
	email    = "token_immutable@localhost.localdomain"
	password = "passw0rd"
}

resource "forgejo_access_token" "test" {
	username = forgejo_user.test.login
	name     = "immutable-test-updated"
	scopes   = ["read:repository"]
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("name"), knownvalue.StringExact("immutable-test-updated")),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccAccessTokenResource_TokenPersistsAfterRead(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckAccessToken(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create token - should have token value in state
			{
				Config: providerConfig + `
resource "forgejo_user" "test" {
	login    = "token_persist_user"
	email    = "token_persist@localhost.localdomain"
	password = "passw0rd"
}

resource "forgejo_access_token" "test" {
	username = forgejo_user.test.login
	name     = "persist-test"
	scopes   = ["read:user"]
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("token"), knownvalue.NotNull()),
				},
			},
			// Second apply with same config - token should still be in state
			// This validates that the token value is preserved when the API returns empty
			{
				Config: providerConfig + `
resource "forgejo_user" "test" {
	login    = "token_persist_user"
	email    = "token_persist@localhost.localdomain"
	password = "passw0rd"
}

resource "forgejo_access_token" "test" {
	username = forgejo_user.test.login
	name     = "persist-test"
	scopes   = ["read:user"]
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("token"), knownvalue.NotNull()),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccAccessTokenResource_WithRepositories(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckAccessToken(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create a token limited to a single repository
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "token_repos_org"
}

resource "forgejo_repository" "test" {
	owner = forgejo_organization.test.name
	name  = "token_repos_repo"
}

resource "forgejo_user" "test" {
	login    = "token_repos_user"
	email    = "token_repos@localhost.localdomain"
	password = "passw0rd"
}

resource "forgejo_collaborator" "test" {
	repository_id = forgejo_repository.test.id
	user          = forgejo_user.test.login
	permission    = "read"
}

resource "forgejo_access_token" "test" {
	username     = forgejo_user.test.login
	name         = "test-token-repos"
	scopes       = ["read:repository"]
	repositories = ["${forgejo_repository.test.full_name}"]

	depends_on = [forgejo_collaborator.test]
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("token"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("repositories"), knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact("token_repos_org/token_repos_repo"),
					})),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccAccessTokenResource_InvalidRepository(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckAccessToken(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "forgejo_access_token" "test" {
	username     = "tfadmin"
	name         = "test-token-invalid-repo"
	scopes       = ["read:repository"]
	repositories = ["no-slash"]
}
`,
				ExpectError: regexp.MustCompile("must be of the form owner/name"),
			},
		},
	})
}
