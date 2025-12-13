# Organization
resource "forgejo_organization" "test" {
  name = "test_org"
}

# Team with admin access
resource "forgejo_team" "admins" {
  organization = forgejo_organization.test.name
  name         = "Admins"
  description  = "Administrator team"
  is_admin     = true
}
