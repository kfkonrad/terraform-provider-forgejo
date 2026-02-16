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

# Repository to protect with branch rules
resource "forgejo_repository" "example" {
  name      = "branch_rule_demo_repo"
  auto_init = true
}

# Basic branch rule protecting the main branch
resource "forgejo_repository_branch_rule" "main" {
  repository               = forgejo_repository.example.full_name
  protected_branch_pattern = "main"
  enable_push              = true
}
