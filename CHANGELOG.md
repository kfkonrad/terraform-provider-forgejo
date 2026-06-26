## 0.11.0

FEATURES:

- **New Resource**: `forgejo_organization_runner` ([documentation](docs/resources/organization_runner.md))
- **New Resource**: `forgejo_repository_runner` ([documentation](docs/resources/repository_runner.md))
- **New Resource**: `forgejo_repository_label` ([documentation](docs/resources/repository_label.md))

## 0.10.0 (February 20, 2026)

FEATURES:

- **New Resource**: `forgejo_ssh_key` ([documentation](docs/resources/ssh_key.md))
- **New Data Source**: `forgejo_ssh_key` ([documentation](docs/data-sources/ssh_key.md))
- **New Resource**: `forgejo_gpg_key` ([documentation](docs/resources/gpg_key.md))
- **New Data Source**: `forgejo_gpg_key` ([documentation](docs/data-sources/gpg_key.md))
- **New Resource**: `forgejo_repository_branch_rule` ([documentation](docs/resources/repository_branch_rule.md))
- **New Resource**: `forgejo_repository_tag_rule` ([documentation](docs/resources/repository_tag_rule.md))
- **New Resource**: `forgejo_organization_action_variable` ([documentation](docs/resources/organization_action_variable.md))
- **New Resource**: `forgejo_repository_action_variable` ([documentation](docs/resources/repository_action_variable.md))

ENHANCEMENTS:

- `forgejo_repository`: allow archiving repositories on destroy instead of deleting them ([documentation](docs/resources/repository.md))
- `forgejo_repository`: add `fast-forward-only` option to the default merge style validator
- `forgejo_team`, `forgejo_team_membership`, `forgejo_collaborator`: simplify import IDs
- Run acceptance tests against OpenTofu as well as Terraform, and against Terraform 1.14

BUG FIXES:

- Add nil-safety checks and standardize error messages across resources
- Standardize on formatting temporal data in RFC3339 format
- `forgejo_organization_action_variable`, `forgejo_repository_action_variable`: fix import
- `forgejo_organization_action_secret`, `forgejo_repository_action_secret`: implement proper deletion
- `forgejo_repository` (data source): simplify schema for consistency (**breaking change** — users upgrading will need to update their configurations) ([documentation](docs/data-sources/repository.md))

DEPENDENCIES:

- Bump dependencies

## 0.9.0 (February 11, 2026)

FEATURES:

- **New Resource**: `forgejo_repository_webhook` ([documentation](docs/resources/repository_webhook.md))

ENHANCEMENTS:

- `forgejo_repository_webhook`: use named events instead of a list
- `forgejo_access_token`: switch `scopes` from a list to a set
- Improve documentation of multiple resources

## 0.8.0 (December 11, 2025)

ENHANCEMENTS:

- `forgejo_user`: add import support ([documentation](docs/resources/user.md))
- `forgejo_collaborator`: add import support ([documentation](docs/resources/collaborator.md))

BUG FIXES:

- Fix integration tests

## 0.7.0

ENHANCEMENTS:

- `forgejo_repository`: add import support using format `owner:repo_name` ([documentation](docs/resources/repository.md))
- `forgejo_organization`: add import support using format `org_name` ([documentation](docs/resources/organization.md))
- `forgejo_team`: add import support using format `team_id` ([documentation](docs/resources/team.md))
- `forgejo_team_membership`: add import support using format `team_id:username` ([documentation](docs/resources/team_membership.md))

## 0.6.1

BUG FIXES:

- Access Tokens will now remain in terraform state after subsequent applies

## 0.6.0

FEATURES:

- **New Resource**: `forgejo_team` ([documentation](docs/resources/team.md))
- **New Resource**: `forgejo_team_membership` ([documentation](docs/resources/team_membership.md))
- **New Resource**: `forgejo_access_token` ([documentation](docs/resources/access_token.md))
- `forgejo_team`: add granular permission control with `granular_permissions` block

ENHANCEMENTS:

- Provider authentication now supports OTP (One-Time Password) for two-factor authentication
- Improved release process for OpenTofu compatibility

BUG FIXES:

- Release script now correctly handles OpenTofu compatibility
- Goreleaser configuration updated for OpenTofu compatibility

## 0.5.4 (October 26, 2025)

BUG FIXES:

- `forgejo_collaborator`: load correct result field into data model
- `forgejo_deploy_key`, `forgejo_repository`: mark "sticky" attributes, to minimize number of unknown values during plan
- `forgejo_organization_action_secret`, `forgejo_repository_action_secret`: add missing `created_at` attribute ([documentation](docs/resources/organization_action_secret.md))
- `forgejo_organization`: add missing `repo_admin_change_team_access` attribute ([documentation](docs/resources/organization.md))
- `forgejo_user`: only update password if it has changed, to not trigger false notifications
- `forgejo_user`: rename `created` attribute for consistency

## 0.5.3 (October 20, 2025)

BUG FIXES:

- `forgejo_repository`: mark create-only attributes as requiring resource replacement only if configuration value is not null
- `forgejo_user`: add remaining attributes ([documentation](docs/resources/user.md))

## 0.5.2 (October 19, 2025)

BUG FIXES:

- `forgejo_repository`: add `regexp` to allowed attribute values for `external_tracker_style`

ENHANCEMENTS:

- Update local test environment to forgejo:11
- Include Terraform 1.13 and exclude Terraform 1.10 from acceptance tests

DEPENDENCIES:

- Update to go 1.24.9

## 0.5.1 (October 18, 2025)

BUG FIXES:

- `forgejo_repository`: add remaining attributes ([documentation](docs/resources/repository.md))
- `forgejo_repository`: flag create-only attributes with `RequiresReplace`
- `forgejo_repository`: only update pull request settings if PRs are enabled
- `forgejo_repository`: remove default value for `default_branch` attribute, to allow for mirroring repos with non-default branch

ENHANCEMENTS:

- Improve documentation and test cases
- `forgejo_repository`: document dependencies for feature settings ([documentation](docs/resources/repository.md))

DEPENDENCIES:

- Bump github.com/hashicorp/terraform-plugin-framework-validators from 0.18.0 to 0.19.0

## 0.5.0 (October 13, 2025)

FEATURES:

- **New Resource**: `forgejo_repository_action_secret` ([documentation](docs/resources/repository_action_secret.md))
- **New Resource**: `forgejo_organization_action_secret` ([documentation](docs/resources/organization_action_secret.md))

BUG FIXES:

- `forgejo_repository`: improve schema validation for tracker and wiki attributes ([documentation](docs/resources/repository.md))

ENHANCEMENTS:

- Add more test cases
- Improve documentation

DEPENDENCIES:

- Bump actions/setup-go from 5.5.0 to 6.0.0
- Bump github.com/hashicorp/terraform-plugin-framework from 1.15.1 to 1.16.1
- Bump github.com/hashicorp/terraform-plugin-go from 0.28.0 to 0.29.0

## 0.4.0 (August 24, 2025)

FEATURES:

- **New Resource**: `forgejo_collaborator` ([documentation](docs/resources/collaborator.md))
- **New Data Source**: `forgejo_collaborator` ([documentation](docs/data-sources/collaborator.md))

DEPENDENCIES:

- Update to Go version 1.24.6
- Bump actions/checkout from 4.2.2 to 5.0.0
- Bump github.com/hashicorp/terraform-plugin-framework 1.15.0 to 1.15.1
- Bump github.com/hashicorp/terraform-plugin-testing from 1.13.2 to 1.13.3
- Bump goreleaser/goreleaser-action from 6.3.0 to 6.4.0

## 0.3.1 (June 29, 2025)

FEATURES:

- `forgejo_repository`: add token authentication for repository migrations (clone & pull mirror repos) ([documentation](docs/resources/repository.md))

DEPENDENCIES:

- Update to go 1.23.10
- Bump codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2 from 2.1.0 to 2.2.0
- Bump github.com/cloudflare/circl from 1.6.0 to 1.6.1
- Bump github.com/cloudflare/circl from 1.3.7 to 1.6.1 in /tools
- Bump github.com/hashicorp/terraform-plugin-testing from 1.13.1 to 1.13.2

## 0.3.0 (June 8, 2025)

FEATURES:

- `forgejo_repository`: add support for repository migration (clone & pull mirror repos) ([documentation](docs/resources/repository.md))

ENHANCEMENTS:

- Automatically run acceptance tests in CI

DOCUMENTATION:

- Add open in dev container badge

DEPENDENCIES:

- Use Go version 1.23.9 consistently
- Update test environment to mariadb:lts
- Remove unneeded dependencies
- Bump github.com/hashicorp/terraform-plugin-go from 0.27.0 to 0.28.0
- Bump github.com/hashicorp/terraform-plugin-testing from 1.13.0 to 1.13.1

## 0.2.4 (May 24, 2025)

DOCUMENTATION:

- Add workflow status badges
- Update examples to use variables instead of hardcoded secrets

DEPENDENCIES:

- Update local test environment to forgejo:10.0
- Migrate golangci-lint configuration from v1 to v2
- Bump actions/setup-go from 5.3.0 to 5.5.0
- Bump codeberg.org/mvdkleijn/forgejo-sdk/forgejo from v2.0.0 to v2.1.0
- Bump crazy-max/ghaction-import-gpg from 6.2.0 to 6.3.0
- Bump github.com/hashicorp/terraform-plugin-framework
- Bump github.com/hashicorp/terraform-plugin-framework-validators
- Bump github.com/hashicorp/terraform-plugin-testing
- Bump golang.org/x/net from 0.36.0 to 0.38.0
- Bump golang.org/x/net from 0.36.0 to 0.38.0 in /tools
- Bump golangci/golangci-lint-action from 6.5.1 to 8.0.0
- Bump goreleaser/goreleaser-action from 6.2.1 to 6.3.0

## 0.2.3 (March 22, 2025)

ENHANCEMENTS:

- Improve documentation

DEPENDENCIES:

- Bump golang.org/x/net from 0.33.0 to 0.36.0 in /tools
- Bump golang.org/x/net from 0.34.0 to 0.36.0
- Bump golangci/golangci-lint-action from 6.5.0 to 6.5.1
- Bump github.com/golang-jwt/jwt/v4 in /tools

## 0.2.2 (March 1, 2025)

DEPENDENCIES:

- Update golangci-lint config from template repository

## 0.2.1 (March 1, 2025)

ENHANCEMENTS:

- Improve documentation

DEPENDENCIES:

- Update local test environment to forgejo:9.0 & mariadb:10
- Update GitHub workflows from template repository
- Bump codeberg.org/mvdkleijn/forgejo-sdk/forgejo from v1.2.0 to v2.0.0
- Bump github.com/hashicorp/terraform-plugin-framework from 1.13.0 to 1.14.1
- Bump github.com/hashicorp/terraform-plugin-framework-validators from 0.16.0 to 0.17.0
- Bump golangci/golangci-lint-action from 6.3.1 to 6.5.0
- Bump goreleaser/goreleaser-action from 6.2.0 to 6.2.1

## 0.2.0 (February 8, 2025)

FEATURES:

- **New Resource**: `forgejo_deploy_key` ([documentation](docs/resources/deploy_key.md))
- **New Data Source**: `forgejo_deploy_key` ([documentation](docs/data-sources/deploy_key.md))

DEPENDENCIES:

- Bump golangci/golangci-lint-action from 6.1.1 to 6.3.0
- Bump github.com/hashicorp/terraform-plugin-go from 0.25.0 to 0.26.0
- Bump actions/setup-go from 5.2.0 to 5.3.0
- Bump golang.org/x/net from 0.23.0 to 0.33.0 in /tools

## 0.1.2 (January 17, 2025)

ENHANCEMENTS:

- Improve documentation and add troubleshooting section

DEPENDENCIES:

- Bump golang.org/x/crypto from 0.21.0 to 0.31.0 in /tools
- Bump github.com/golang-jwt/jwt/v4 in /tools
- Bump golang.org/x/crypto from 0.29.0 to 0.31.0
- Bump goreleaser/goreleaser-action from 6.0.0 to 6.1.0
- Bump golangci/golangci-lint-action from 6.1.0 to 6.1.1
- Bump crazy-max/ghaction-import-gpg from 6.1.0 to 6.2.0
- Bump actions/setup-go from 5.0.2 to 5.2.0
- Bump actions/checkout from 4.1.7 to 4.2.2

## 0.1.1 (December 24, 2024) 🎄

ENHANCEMENTS:

- Improve documentation and examples

## 0.1.0 (December 23, 2024)

MINIMUM VIABLE PRODUCT (MVP)

FEATURES:

- Authentication with API token, or with username and password
- **New Resource**: `forgejo_organization` ([documentation](docs/resources/organization.md))
- **New Resource**: `forgejo_repository` ([documentation](docs/resources/repository.md))
- **New Resource**: `forgejo_user` ([documentation](docs/resources/user.md))
- **New Data Source**: `forgejo_organization` ([documentation](docs/data-sources/organization.md))
- **New Data Source**: `forgejo_repository` ([documentation](docs/data-sources/repository.md))
- **New Data Source**: `forgejo_user` ([documentation](docs/data-sources/user.md))
