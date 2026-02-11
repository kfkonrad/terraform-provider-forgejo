# Webhook with secret and custom headers
resource "forgejo_repository_webhook" "secure" {
  repository = forgejo_repository.example.full_name
  type       = "gitea"
  url        = "https://api.example.com/webhooks/forgejo"
  events {
    push         = true
    pull_request = true
  }
  active               = true
  secret               = "my-webhook-secret-12345"
  authorization_header = "Bearer my-api-token-67890"
  content_type         = "json"
  branch_filter        = "main/*,release/*"
}

# Form-encoded webhook
resource "forgejo_repository_webhook" "form_encoded" {
  repository = forgejo_repository.example.full_name
  type       = "forgejo"
  url        = "https://legacy-api.example.com/webhook"
  events {
    push = true
  }
  active       = true
  content_type = "form"
}
