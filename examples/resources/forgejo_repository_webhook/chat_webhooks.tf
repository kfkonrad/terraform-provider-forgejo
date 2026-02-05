# Slack webhook with additional configuration
resource "forgejo_repository_webhook" "slack" {
  repository = forgejo_repository.example.full_name
  type       = "slack"
  url        = "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK"
  events     = ["push", "pull_request", "issues", "release"]
  active     = true
  config = {
    channel  = "#general"
    username = "forgejo-bot"
    icon_url = "https://example.com/forgejo-icon.png"
  }
}

# Discord webhook
resource "forgejo_repository_webhook" "discord" {
  repository = forgejo_repository.example.full_name
  type       = "discord"
  url        = "https://discord.com/api/webhooks/YOUR/DISCORD/WEBHOOK"
  events     = ["push", "pull_request"]
  active     = true
  config = {
    username = "Forgejo Bot"
    icon_url = "https://example.com/forgejo-icon.png"
  }
}