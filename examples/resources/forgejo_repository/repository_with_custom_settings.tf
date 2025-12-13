# Repository with custom settings
resource "forgejo_repository" "custom" {
  name           = "custom_repository"
  description    = "A repository with custom configuration"
  website        = "https://example.com"
  private        = true
  template       = false
  default_branch = "main"
  issue_labels   = "Default"
  auto_init      = true
  readme         = "Default"
  trust_model    = "collaborator"
  archived       = false

  has_issues        = true
  has_wiki          = true
  has_pull_requests = true
  has_projects      = true

  external_tracker = {
    external_tracker_url    = "https://github.com/example/repo/issues"
    external_tracker_format = "https://github.com/example/repo/issues/{index}"
    external_tracker_style  = "numeric"
  }
}
