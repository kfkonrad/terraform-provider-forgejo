variable "github_token" {
  sensitive = true
}

# Clone an external repository
resource "forgejo_repository" "clone" {
  name       = "cloned_repo"
  clone_addr = "https://github.com/example/repository"
  auth_token = var.github_token
  mirror     = false
}
