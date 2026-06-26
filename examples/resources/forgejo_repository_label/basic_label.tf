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

# Repository to attach labels to
resource "forgejo_repository" "example" {
  name      = "label_demo_repo"
  auto_init = true
}

# Basic issue label
resource "forgejo_repository_label" "bug" {
  repository  = forgejo_repository.example.full_name
  name        = "bug"
  color       = "d73a4a"
  description = "Something isn't working"
}
