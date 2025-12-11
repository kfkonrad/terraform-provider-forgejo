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

# Existing user
data "forgejo_user" "this" {
  login = "test_user"
}
