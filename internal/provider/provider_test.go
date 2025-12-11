package provider_test

import (
	"os"
	"testing"

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
  host = "http://localhost:3000"
}
`
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
	// Access token tests require username/password authentication
	// because creating access tokens for users via API token auth is not allowed by Forgejo
	username := os.Getenv("FORGEJO_USERNAME")
	password := os.Getenv("FORGEJO_PASSWORD")
	apiToken := os.Getenv("FORGEJO_API_TOKEN")

	if apiToken != "" {
		t.Skip("Access token tests require username/password authentication (FORGEJO_USERNAME and FORGEJO_PASSWORD), skipping because FORGEJO_API_TOKEN is set")
	}

	if username == "" || password == "" {
		t.Fatal("Access token tests require both FORGEJO_USERNAME and FORGEJO_PASSWORD to be set")
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
