package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAccessTokenResource_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
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
		PreCheck:                 func() { testAccPreCheck(t) },
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
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("scopes"), knownvalue.ListExact([]knownvalue.Check{
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
		PreCheck:                 func() { testAccPreCheck(t) },
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
		PreCheck:                 func() { testAccPreCheck(t) },
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
