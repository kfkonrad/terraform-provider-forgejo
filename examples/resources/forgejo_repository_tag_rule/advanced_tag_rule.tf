# Advanced tag rule with whitelist restrictions
resource "forgejo_repository_tag_rule" "releases" {
  repository            = forgejo_repository.example.full_name
  protected_tag_pattern = "release/**"

  # Only allow specific users and teams to create matching tags
  whitelist_usernames = ["release-manager"]
  whitelist_teams     = ["release-team"]
}
