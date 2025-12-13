# Organization
resource "forgejo_organization" "test" {
  name = "test_org"
}

# Team with granular permissions
resource "forgejo_team" "developers" {
  organization = forgejo_organization.test.name
  name         = "Developers"
  description  = "Development team"
  is_admin     = false

  permissions = {
    code       = "write"
    issues     = "write"
    pulls      = "write"
    releases   = "read"
    wiki       = "read"
    ext_wiki   = "none"
    ext_issues = "none"
    projects   = "write"
    packages   = "read"
    actions    = "write"
  }

  can_create_org_repo       = true
  includes_all_repositories = false
}
