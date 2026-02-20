# Advanced branch rule with full protections
resource "forgejo_repository_branch_rule" "strict" {
  repository               = forgejo_repository.example.full_name
  protected_branch_pattern = "release/**"

  # Push restrictions
  enable_push              = true
  enable_push_whitelist    = true
  push_whitelist_usernames = ["release-manager"]
  push_whitelist_teams     = ["release-team"]

  # Require approvals
  required_approvals            = 2
  enable_approvals_whitelist    = true
  approvals_whitelist_usernames = ["lead-dev", "senior-dev"]
  dismiss_stale_approvals       = true

  # Status checks
  enable_status_check   = true
  status_check_contexts = ["ci/build", "ci/test"]

  # Merge restrictions
  enable_merge_whitelist            = true
  merge_whitelist_usernames         = ["release-manager"]
  block_on_rejected_reviews         = true
  block_on_official_review_requests = true
  block_on_outdated_branch          = true

  # File protection
  protected_file_patterns   = "LICENSE;go.mod;go.sum"
  unprotected_file_patterns = "docs/**"
}
