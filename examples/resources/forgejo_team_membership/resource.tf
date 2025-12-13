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

# Organization
resource "forgejo_organization" "test" {
  name = "test_org"
}

# Team
resource "forgejo_team" "developers" {
  organization = forgejo_organization.test.name
  name         = "Developers"
  is_admin     = false

  permissions = {
    code   = "write"
    issues = "write"
    pulls  = "write"
  }
}

# User
resource "forgejo_user" "developer" {
  username = "developer1"
  email    = "developer1@example.com"
  password = "dev123pass"
}

# Team membership
resource "forgejo_team_membership" "dev_member" {
  team_id  = forgejo_team.developers.id
  username = forgejo_user.developer.username
}
