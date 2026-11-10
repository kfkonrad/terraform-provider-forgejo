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

# Scoped, mutually exclusive labels: an issue can carry only one "priority/" label
resource "forgejo_repository_label" "priority_high" {
  repository = forgejo_repository.example.full_name
  name       = "priority/high"
  color      = "b60205"
  exclusive  = true
}

resource "forgejo_repository_label" "priority_low" {
  repository = forgejo_repository.example.full_name
  name       = "priority/low"
  color      = "0e8a16"
  exclusive  = true
}

# Archived label: kept for history but no longer offered for new issues
resource "forgejo_repository_label" "legacy" {
  repository  = forgejo_repository.example.full_name
  name        = "legacy"
  color       = "cccccc"
  description = "No longer used"
  is_archived = true
}
