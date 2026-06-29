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

# Organization the runner is scoped to
resource "forgejo_organization" "example" {
  name = "runner-demo-org"
}

# Generate a runner registration token at organization scope.
# Pass the token to a runner daemon to register; the token is single-use.
resource "forgejo_organization_runner" "ci" {
  organization = forgejo_organization.example.name
}

output "runner_token" {
  value     = forgejo_organization_runner.ci.token
  sensitive = true
}
