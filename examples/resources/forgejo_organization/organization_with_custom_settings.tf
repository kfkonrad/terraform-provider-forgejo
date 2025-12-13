# Organization with custom settings
resource "forgejo_organization" "custom" {
  name        = "custom_org"
  full_name   = "My Custom Organization"
  description = "An organization with custom settings"
  website     = "https://example.com"
  location    = "San Francisco"
  visibility  = "private"
}
