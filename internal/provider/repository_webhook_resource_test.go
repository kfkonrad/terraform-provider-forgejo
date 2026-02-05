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

func TestAccRepositoryWebhookResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing - basic webhook
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name = "test_webhook_repo"
}
resource "forgejo_repository_webhook" "test" {
	repository = forgejo_repository.test.full_name
	type       = "gitea"
	url        = "http://example.com/webhook"
	events     = ["push", "pull_request"]
	active     = true
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("repository"), knownvalue.StringExact("tfadmin/test_webhook_repo")),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("owner"), knownvalue.StringExact("tfadmin")),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("type"), knownvalue.StringExact("gitea")),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("url"), knownvalue.StringExact("http://example.com/webhook")),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("content_type"), knownvalue.StringExact("json")),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("events"), knownvalue.ListSizeExact(2)),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("active"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("created_at"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("updated_at"), knownvalue.NotNull()),
				},
			},
			// Update testing - change events and active status
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name = "test_webhook_repo"
}
resource "forgejo_repository_webhook" "test" {
	repository = forgejo_repository.test.full_name
	type       = "gitea"
	url        = "http://example.com/webhook"
	events     = ["push", "pull_request", "issues"]
	active     = false
	branch_filter = "main/*"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("events"), knownvalue.ListSizeExact(3)),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("active"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("branch_filter"), knownvalue.StringExact("main/*")),
				},
			},
			// Update testing - add secret and authorization header
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name = "test_webhook_repo"
}
resource "forgejo_repository_webhook" "test" {
	repository           = forgejo_repository.test.full_name
	type                 = "gitea"
	url                  = "http://example.com/webhook"
	events               = ["push", "pull_request", "issues"]
	active               = true
	secret               = "mysecret123"
	authorization_header = "Bearer token123"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("active"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("branch_filter"), knownvalue.Null()),
				},
			},
			// Import testing
			{
				ResourceName: "forgejo_repository_webhook.test",
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["forgejo_repository_webhook.test"]
					webhookID := rs.Primary.Attributes["id"]
					owner := rs.Primary.Attributes["owner"]

					// Parse repository to get owner/repo format
					return fmt.Sprintf("%s:%s:%s", owner, "test_webhook_repo", webhookID), nil
				},
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret", "authorization_header"}, // Sensitive fields
			},
		},
	})
}

func TestAccRepositoryWebhookResource_Slack(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing - Slack webhook with config
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name = "test_slack_webhook_repo"
}
resource "forgejo_repository_webhook" "test" {
	repository = forgejo_repository.test.full_name
	type       = "slack"
	url        = "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX"
	events     = ["push", "pull_request"]
	active     = true
	config = {
		channel = "#general"
		username = "forgejo-bot"
		icon_url = "https://example.com/icon.png"
	}
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("repository"), knownvalue.StringExact("tfadmin/test_slack_webhook_repo")),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("owner"), knownvalue.StringExact("tfadmin")),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("type"), knownvalue.StringExact("slack")),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("url"), knownvalue.StringExact("https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX")),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("events"), knownvalue.ListSizeExact(2)),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("active"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("config"), knownvalue.NotNull()),
				},
			},
		},
	})
}

func TestAccRepositoryWebhookResource_Discord(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing - Discord webhook
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name = "test_discord_webhook_repo"
}
resource "forgejo_repository_webhook" "test" {
	repository = forgejo_repository.test.full_name
	type       = "discord"
	url        = "https://discord.com/api/webhooks/1234567890/ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	events     = ["push"]
	active     = true
	content_type = "json"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("type"), knownvalue.StringExact("discord")),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("events"), knownvalue.ListSizeExact(1)),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("content_type"), knownvalue.StringExact("json")),
				},
			},
		},
	})
}

func TestAccRepositoryWebhookResource_FormContentType(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing - Form content type
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name = "test_form_webhook_repo"
}
resource "forgejo_repository_webhook" "test" {
	repository    = forgejo_repository.test.full_name
	type          = "forgejo"
	url           = "http://example.com/webhook"
	events        = ["push"]
	active        = true
	content_type  = "form"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("type"), knownvalue.StringExact("forgejo")),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("content_type"), knownvalue.StringExact("form")),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("events"), knownvalue.ListSizeExact(1)),
				},
			},
		},
	})
}

func TestAccRepositoryWebhookResource_OrganizationRepository(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing - Organization repository
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "test-webhook-org"
}
resource "forgejo_repository" "test" {
	owner = forgejo_organization.test.name
	name  = "test_repo"
}
resource "forgejo_repository_webhook" "test" {
	repository = forgejo_repository.test.full_name
	type       = "gitea"
	url        = "http://example.com/webhook"
	events     = ["push", "pull_request"]
	active     = true
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("repository"), knownvalue.StringExact("test-webhook-org/test_repo")),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("owner"), knownvalue.StringExact("test-webhook-org")),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("type"), knownvalue.StringExact("gitea")),
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("events"), knownvalue.ListSizeExact(2)),
				},
			},
			// Import testing for organization repository
			{
				ResourceName: "forgejo_repository_webhook.test",
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["forgejo_repository_webhook.test"]
					webhookID := rs.Primary.Attributes["id"]
					owner := rs.Primary.Attributes["owner"]

					// Parse repository to get owner/repo format
					return fmt.Sprintf("%s:%s:%s", owner, "test_repo", webhookID), nil
				},
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret", "authorization_header"}, // Sensitive fields
			},
		},
	})
}

func TestAccRepositoryWebhookResource_DefaultEvents(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create without specifying events (should default to common events)
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name = "test_default_events_repo"
}
resource "forgejo_repository_webhook" "test" {
	repository = forgejo_repository.test.full_name
	type       = "gitea"
	url        = "http://example.com/webhook"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("events"), knownvalue.ListSizeExact(4)), // Default events: push, pull_request, issues, release
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("active"), knownvalue.Bool(true)),       // Default active
				},
			},
		},
	})
}

func TestAccRepositoryWebhookResource_TypeRequiresReplace(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with gitea type
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name = "test_replace_type_repo"
}
resource "forgejo_repository_webhook" "test" {
	repository = forgejo_repository.test.full_name
	type       = "gitea"
	url        = "http://example.com/webhook"
	events     = ["push"]
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("type"), knownvalue.StringExact("gitea")),
				},
			},
			// Change type to slack - should force replacement
			{
				Config: providerConfig + `
resource "forgejo_repository" "test" {
	name = "test_replace_type_repo"
}
resource "forgejo_repository_webhook" "test" {
	repository = forgejo_repository.test.full_name
	type       = "slack"
	url        = "http://example.com/webhook"
	events     = ["push"]
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_repository_webhook.test", tfjsonpath.New("type"), knownvalue.StringExact("slack")),
				},
				ExpectNonEmptyPlan: true, // Should trigger recreation
			},
		},
	})
}
