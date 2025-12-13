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

# User
resource "forgejo_user" "test" {
  username = "testuser"
  email    = "testuser@example.com"
  password = "testpass123"
}

# Access token
resource "forgejo_access_token" "this" {
  username = forgejo_user.test.username
  name     = "my_api_token"
  scopes   = ["repo", "write:org"]
}
