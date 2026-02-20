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

# Organization action variable
resource "forgejo_organization_action_variable" "this" {
  organization = forgejo_organization.test.name
  name         = "my_variable"
  value        = "my_variable_value"
}
