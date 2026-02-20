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
