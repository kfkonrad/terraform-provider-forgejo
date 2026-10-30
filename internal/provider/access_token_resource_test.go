package provider_test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v4"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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

// TestAccAccessTokenResource_TokenAuth manages tokens while the provider
// authenticates with an API token, which goes through the admin API.
func TestAccAccessTokenResource_TokenAuth(t *testing.T) {
	testAccPreCheck(t)
	testAccSkipUnlessForgejo(t, ">= 16.0.0")
	testAccUseAPIToken(t)

	userConfig := `
resource "forgejo_organization" "test" {
	name = "token_auth_org"
}

resource "forgejo_repository" "test" {
	owner = forgejo_organization.test.name
	name  = "token_auth_repo"
}

resource "forgejo_user" "test" {
	login                = "token_auth_user"
	email                = "token_auth@localhost.localdomain"
	password             = "passw0rd"
	must_change_password = false
}

resource "forgejo_collaborator" "test" {
	repository_id = forgejo_repository.test.id
	user          = forgejo_user.test.login
	permission    = "read"
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create a token for another user without basic auth
			{
				Config: providerConfig + userConfig + `
resource "forgejo_access_token" "test" {
	username = forgejo_user.test.login
	name     = "token-auth"
	scopes   = ["read:user"]
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("username"), knownvalue.StringExact("token_auth_user")),
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("scopes"), knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact("read:user"),
					})),
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("repositories"), knownvalue.Null()),
				},
				Check: resource.TestCheckResourceAttrWith("forgejo_access_token.test", "token", testAccCheckTokenAuthenticatesAs("token_auth_user")),
			},
			// Replace it with a token limited to one repository; the old one
			// is deleted through the admin API
			{
				Config: providerConfig + userConfig + `
resource "forgejo_access_token" "test" {
	username     = forgejo_user.test.login
	name         = "token-auth-repos"
	scopes       = ["read:repository"]
	repositories = [forgejo_repository.test.full_name]

	depends_on = [forgejo_collaborator.test]
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("name"), knownvalue.StringExact("token-auth-repos")),
					statecheck.ExpectKnownValue("forgejo_access_token.test", tfjsonpath.New("repositories"), knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact("token_auth_org/token_auth_repo"),
					})),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrWith("forgejo_access_token.test", "token", testAccCheckTokenCanReadRepo("token_auth_org", "token_auth_repo")),
					testAccCheckAccessTokenCount("token_auth_user", 1),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

// TestAccAccessTokenResource_TokenAuthUnsupported checks the error when the
// provider authenticates with an API token against a Forgejo without the
// admin access token API.
func TestAccAccessTokenResource_TokenAuthUnsupported(t *testing.T) {
	testAccPreCheck(t)
	testAccSkipUnlessForgejo(t, "< 16.0.0")
	testAccUseAPIToken(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "forgejo_access_token" "test" {
	username = "tfadmin"
	name     = "token-auth-unsupported"
	scopes   = ["read:user"]
}
`,
				ExpectError: regexp.MustCompile(`requires Forgejo 16 or newer`),
			},
		},
	})
}

// testAccUseAPIToken makes the provider under test authenticate with an API
// token. When the suite runs with basic auth, a site admin token is created
// for the duration of the test.
func testAccUseAPIToken(t *testing.T) {
	t.Helper()

	if os.Getenv("FORGEJO_API_TOKEN") != "" {
		return
	}

	c := testAccClient(t)
	username := os.Getenv("FORGEJO_USERNAME")
	adminToken, _, err := c.CreateAccessToken(username, forgejo.CreateAccessTokenOption{
		Name:   "tf-acc-" + t.Name(),
		Scopes: []forgejo.AccessTokenScope{forgejo.AccessTokenScopeAll},
	})
	if err != nil {
		t.Fatalf("Unable to create admin token: %s", err)
	}
	t.Cleanup(func() {
		if _, err := c.DeleteAccessTokenByID(context.Background(), username, adminToken.ID); err != nil {
			t.Errorf("Unable to delete admin token: %s", err)
		}
	})

	t.Setenv("FORGEJO_USERNAME", "")
	t.Setenv("FORGEJO_PASSWORD", "")
	t.Setenv("FORGEJO_OTP", "")
	t.Setenv("FORGEJO_API_TOKEN", adminToken.Token)
}

// testAccCheckTokenAuthenticatesAs checks that a token value is accepted by
// Forgejo and belongs to the given user.
func testAccCheckTokenAuthenticatesAs(username string) resource.CheckResourceAttrWithFunc {
	return func(token string) error {
		c, err := forgejo.NewClient(testAccHost, forgejo.SetToken(token))
		if err != nil {
			return err
		}
		user, _, err := c.GetMyUserInfo()
		if err != nil {
			return fmt.Errorf("token not accepted: %w", err)
		}
		if user.UserName != username {
			return fmt.Errorf("token belongs to %q, expected %q", user.UserName, username)
		}

		return nil
	}
}

// testAccCheckTokenCanReadRepo checks that a token value grants read access
// to the given repository.
func testAccCheckTokenCanReadRepo(owner, repo string) resource.CheckResourceAttrWithFunc {
	return func(token string) error {
		c, err := forgejo.NewClient(testAccHost, forgejo.SetToken(token))
		if err != nil {
			return err
		}
		if _, _, err := c.GetRepo(owner, repo); err != nil {
			return fmt.Errorf("token cannot read %s/%s: %w", owner, repo, err)
		}

		return nil
	}
}

// testAccCheckAccessTokenCount checks how many access tokens a user has,
// using the admin API with the credentials the provider uses.
func testAccCheckAccessTokenCount(username string, want int) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		c, err := forgejo.NewClient(testAccHost, forgejo.SetToken(os.Getenv("FORGEJO_API_TOKEN")))
		if err != nil {
			return err
		}
		tokens, _, err := c.AdminListUserAccessTokens(username, forgejo.ListAccessTokensOptions{})
		if err != nil {
			return err
		}
		if len(tokens) != want {
			return fmt.Errorf("user %q has %d access tokens, expected %d", username, len(tokens), want)
		}

		return nil
	}
}
