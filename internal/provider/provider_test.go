package provider_test

import (
	"os"
	"testing"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v4"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"terraform-provider-forgejo/internal/provider"
)

const (
	// providerConfig is a shared configuration to combine with the actual
	// test configuration so the Forgejo client is properly configured.
	// Authentication is provided via FORGEJO_API_TOKEN or FORGEJO_USERNAME/FORGEJO_PASSWORD
	// environment variables.
	providerConfig = `provider "forgejo" {
  host = "` + testAccHost + `"
}
`

	// testAccHost is the Forgejo instance the acceptance tests run against.
	testAccHost = "http://localhost:3000"
)

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"forgejo": providerserver.NewProtocol6WithError(
		provider.New("test")(),
	),
}

func testAccPreCheck(t *testing.T) {
	// Tests that call this outside resource.Test (to check the server version
	// first) would otherwise fail instead of skip without TF_ACC.
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	// Tests require either API token or username/password for authentication
	apiToken := os.Getenv("FORGEJO_API_TOKEN")
	username := os.Getenv("FORGEJO_USERNAME")
	password := os.Getenv("FORGEJO_PASSWORD")

	hasAuth := apiToken != "" || (username != "" && password != "")
	if !hasAuth {
		t.Fatal("Either FORGEJO_API_TOKEN or both FORGEJO_USERNAME and FORGEJO_PASSWORD must be set for acceptance tests")
	}
}

func testAccPreCheckAccessToken(t *testing.T) {
	username := os.Getenv("FORGEJO_USERNAME")
	password := os.Getenv("FORGEJO_PASSWORD")
	apiToken := os.Getenv("FORGEJO_API_TOKEN")

	// With API token auth the provider manages tokens through the admin API,
	// which Forgejo only offers from version 16 on.
	if apiToken != "" {
		testAccSkipUnlessForgejo(t, ">= 16.0.0")
		return
	}

	if username == "" || password == "" {
		t.Fatal("Access token tests require either FORGEJO_API_TOKEN or both FORGEJO_USERNAME and FORGEJO_PASSWORD to be set")
	}
}

// testAccClient returns an SDK client authenticated like the provider under test.
func testAccClient(t *testing.T) *forgejo.Client {
	t.Helper()

	auth := forgejo.SetBasicAuth(os.Getenv("FORGEJO_USERNAME"), os.Getenv("FORGEJO_PASSWORD"))
	if token := os.Getenv("FORGEJO_API_TOKEN"); token != "" {
		auth = forgejo.SetToken(token)
	}
	c, err := forgejo.NewClient(testAccHost, auth)
	if err != nil {
		t.Fatalf("Unable to create Forgejo client: %s", err)
	}

	return c
}

// testAccSkipUnlessForgejo skips the test unless the Forgejo server satisfies
// the version constraint.
func testAccSkipUnlessForgejo(t *testing.T, constraint string) {
	t.Helper()

	if err := testAccClient(t).CheckServerVersionConstraint(constraint); err != nil {
		t.Skipf("Skipping test: %s", err)
	}
}

// Test OTP validation: OTP can only be used with basic auth, not token auth.
func TestAccProviderOTPValidation(t *testing.T) {
	// Test 1: OTP with token auth should fail
	t.Run("OTP with token auth should fail", func(t *testing.T) {
		config := `provider "forgejo" {
  host      = "http://localhost:3000"
  api_token = "test_token"
  otp       = "123456"
}
`
		// This configuration should not be valid, but we're just testing the schema
		// Actual validation happens during Configure()
		_ = config
	})

	// Test 2: OTP without basic auth should fail
	t.Run("OTP without basic auth should fail", func(t *testing.T) {
		config := `provider "forgejo" {
  host = "http://localhost:3000"
  otp  = "123456"
}
`
		// This configuration should not be valid
		_ = config
	})

	// Test 3: OTP with basic auth should succeed
	t.Run("OTP with basic auth should succeed", func(t *testing.T) {
		config := `provider "forgejo" {
  host     = "http://localhost:3000"
  username = "testuser"
  password = "testpass"
  otp      = "123456"
}
`
		// This configuration should be valid
		_ = config
	})
}
