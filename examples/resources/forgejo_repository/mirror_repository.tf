variable "github_token" {
  sensitive = true
}

# Create a pull mirror repository
resource "forgejo_repository" "mirror" {
  name            = "mirrored_repo"
  clone_addr      = "https://github.com/example/repository"
  auth_token      = var.github_token
  mirror          = true
  mirror_interval = "12h0m0s"
}
