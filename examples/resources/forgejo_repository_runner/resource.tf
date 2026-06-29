terraform {
  required_providers {
    forgejo = {
      source = "kfkonrad/forgejo"
    }
  }
}

provider "forgejo" {
  host = "http://localhost:3000"
}

# Repository the runner is scoped to
resource "forgejo_repository" "example" {
  name      = "runner_demo_repo"
  auto_init = true
}

# Generate a runner registration token at repository scope.
# Pass the token to a runner daemon to register; the token is single-use.
resource "forgejo_repository_runner" "ci" {
  repository = forgejo_repository.example.full_name
}

output "runner_token" {
  value     = forgejo_repository_runner.ci.token
  sensitive = true
}
