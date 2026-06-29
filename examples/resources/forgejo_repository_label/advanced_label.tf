# Multiple labels for triage workflow
resource "forgejo_repository_label" "enhancement" {
  repository  = forgejo_repository.example.full_name
  name        = "enhancement"
  color       = "a2eeef"
  description = "New feature or request"
}

resource "forgejo_repository_label" "good_first_issue" {
  repository  = forgejo_repository.example.full_name
  name        = "good first issue"
  color       = "7057ff"
  description = "Good for newcomers"
}
