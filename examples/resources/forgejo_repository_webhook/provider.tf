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