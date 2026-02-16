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

# Repository to protect with tag rules
resource "forgejo_repository" "example" {
  name      = "tag_rule_demo_repo"
  auto_init = true
}

# Basic tag rule protecting version tags
resource "forgejo_repository_tag_rule" "versions" {
  repository            = forgejo_repository.example.full_name
  protected_tag_pattern = "v*"
}
