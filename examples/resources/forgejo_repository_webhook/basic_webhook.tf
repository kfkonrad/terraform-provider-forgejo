terraform {
  required_providers {
    forgejo = {
      source = "kfkonrad/forgejo"
    }
  }
}

provider "forgejo" {
  host = "http://localhost:3000"
}

# Repository to attach webhook to
resource "forgejo_repository" "example" {
  name = "webhook_demo_repo"
}

# Basic webhook with common events
resource "forgejo_repository_webhook" "basic" {
  repository = forgejo_repository.example.full_name
  type       = "gitea"
  url        = "https://example.com/webhook"
  events {
    push         = true
    pull_request = true
    issues       = true
  }
  active = true
}
