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
	// It is also possible to use the FORGEJO_ environment variables instead,
	// such as updating the Makefile and running the testing through that tool.
	providerConfig = `provider "forgejo" {
  host     = "http://localhost:3000"
  username = "derkev"
  password = "derkev12"
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
