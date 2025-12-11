# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a **Terraform/OpenTofu Provider for Forgejo** — a provider that enables infrastructure-as-code management of Forgejo (self-hosted git forge) resources. The provider is built on the Terraform Plugin Framework and the Forgejo SDK for Go.

## Build, Test, and Development Commands

All commands are defined in `GNUmakefile` and use Go-based tooling:

### Common Development Tasks

```bash
# Default target (format, lint, install)
make

# Build the provider binary
make build

# Build and install the provider binary to $GOPATH/bin
make install

# Run linting with golangci-lint
make lint

# Format code with gofmt
make fmt

# Run unit tests with coverage
make test

# Run acceptance tests (requires TF_ACC=1 environment variable)
# These create real Forgejo resources - used for integration testing
make testacc

# Generate provider documentation from schema
make generate
```

### Running Individual Tests

```bash
# Run tests for a specific package/file
go test -v -cover -timeout=120s ./internal/provider -run TestRepositoryResource

# Run a specific test function
go test -v -cover ./internal/provider -run TestResourceRepositoryRead

# Run tests with verbose output and 10 parallel workers
go test -v -cover -timeout=120s -parallel=10 ./...
```

## Architecture & Code Structure

### High-Level Design

The provider follows the standard Terraform Plugin Framework architecture with a clear separation between data sources (read-only) and resources (CRUD operations):

```
terraform-provider-forgejo/
├── main.go                          # Provider entry point - serves plugin
├── internal/provider/               # All provider logic
│   ├── provider.go                  # Provider configuration & authentication
│   ├── {resource}_resource.go       # Resource implementations (CRUD)
│   ├── {resource}_resource_test.go  # Resource unit tests
│   ├── {resource}_data_source.go    # Data source implementations (read-only)
│   └── {resource}_data_source_test.go
├── tools/                           # Documentation generation
├── docs/                            # Generated Terraform docs
├── examples/                        # Provider usage examples
└── docker/                          # Local Forgejo instance for testing
```

### Core Components

**Provider Package (`internal/provider/`):**
- Pattern: One file pair per resource/data source (e.g., `repository_resource.go` + `repository_resource_test.go`)
- Each resource/data source has a corresponding model struct that maps Terraform schema to Go types (e.g., `repositoryResourceModel`)
- The Forgejo SDK client is injected via the `Configure()` method
- Resources implement `resource.Resource` and `resource.ResourceWithConfigure` interfaces
- Data sources implement `datasource.DataSource` and `datasource.DataSourceWithConfigure` interfaces

**Key Pattern - Resource Implementation:**
1. Define schema in `Schema()` method (describes resource attributes and validation)
2. Define Go model struct with `tfsdk` tags (maps schema to struct fields)
3. Implement CRUD operations: `Create()`, `Read()`, `Update()`, `Delete()`
4. Use Forgejo SDK (`codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2`) for API calls
5. Handle attribute changes and computed fields in `Plan()` method

**Configuration & Authentication (`provider.go`):**
- Supports two auth methods: API token (recommended) or username/password
- Config pulled from Terraform variables or environment variables (`FORGEJO_*`)
- Forgejo API client created in `Configure()` and injected into resources

### Important Implementation Details

- **Create-only attributes**: Attributes like `auto_init`, `gitignores`, `license` cannot be modified after creation. Changes force resource recreation. Use `planmodifier.RequiresReplace()` in schema.
- **Computed fields**: Read-only fields from the API (e.g., `created_at`, `updated_at`, repository URLs) use `Computed: true` in schema
- **Nested objects**: Complex attributes (e.g., `internal_tracker`, `permissions`) use object types with attribute maps
- **Plan modifiers**: Control Terraform's behavior when attributes change (e.g., `boolplanmodifier.RequiresReplace()`)
- **Validators**: Custom validation using `stringvalidator`, `boolvalidator`, etc. from `github.com/hashicorp/terraform-plugin-framework-validators`

## Linting & Code Quality

**Linter Configuration (`.golangci.yml`):**
- Enabled linters: copyloopvar, durationcheck, errcheck, forcetypeassert, godot, ineffassign, makezero, misspell, nilerr, predeclared, staticcheck, unconvert, unparam, unused, usetesting
- Exclusions: generated code, third_party, builtin, examples directories
- Formatter: gofmt with `-s` (simplify) flag
- Run with `make lint`

## Dependencies & Go Version

- **Go Version**: 1.24.0 (with toolchain 1.24.9)
- **Core Dependencies**:
  - `codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2` — Forgejo API client
  - `github.com/hashicorp/terraform-plugin-framework` — Provider framework
  - `github.com/hashicorp/terraform-plugin-testing` — Acceptance testing framework
  - `github.com/hashicorp/terraform-plugin-log` — Logging
- Add dependencies with: `go get <module> && go mod tidy`

### Forgejo SDK Location

The Forgejo SDK source code is cached locally in the Go modules cache at:

```
~/go/pkg/mod/codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2@v2.2.0/
```

Key SDK files for reference:
- `org_team.go` — Team API types and methods (CreateTeamOption, EditTeamOption, Team, RepoUnitType, AccessMode)
- `org.go` — Organization API types and methods
- `repository.go` — Repository API types and methods
- `user.go` — User API types and methods

The current version is **v2.2.0**. Check `go.mod` in the repository root for the exact pinned version.

## Testing

The provider uses two testing levels:

1. **Unit Tests** (`make test`): Fast, local tests that mock API responses
2. **Acceptance Tests** (`make testacc`): Integration tests that create real resources in a Forgejo instance (requires Docker Compose setup in `docker/`)

Test files follow naming pattern: `{resource}_*_test.go`. Tests verify schema validation, CRUD operations, and error handling.

## Documentation Generation

Documentation is auto-generated from resource/data source schemas:
- Run `make generate` to update docs in `docs/`
- Uses tools in `tools/` directory with Go generate
- Generates Terraform registry-compatible markdown in `docs/resources/` and `docs/data-sources/`

**IMPORTANT: Always update documentation when making changes:**
- Run `make generate` after ANY changes to resources or data sources (schema, behavior, features)
- Update README.md when adding new resources, data sources, or significant features
- Update example templates in `examples/` to reflect new functionality
- Document new features like import support, validation rules, or special behaviors
- Keep templates and examples in sync with actual resource schemas
- Outdated documentation will cause user confusion and should be avoided

**When documentation updates are required:**
- Schema changes (adding, modifying, or removing attributes)
- New import functionality added to resources
- Changes to resource behavior or semantics
- New resources or data sources
- Changes to authentication or provider configuration
- New validation rules or constraints
- Breaking changes or deprecations

## Common Development Scenarios

**Adding a new resource:**

1. Create `internal/provider/{resource}_resource.go` with schema, model struct, and CRUD methods
2. Create `internal/provider/{resource}_resource_test.go` with acceptance tests
3. Register resource in `provider.go` Resources() method
4. Add example in `examples/` directory
5. Run `make generate` to create documentation
6. Update README.md to list the new resource
7. Add import support documentation if applicable

**Modifying a resource or data source:**

- Ensure `planmodifier` is set for create-only attributes to prevent unexpected recreation
- Use `Computed: true` for read-only fields from the API
- Run tests to verify backwards compatibility: `make test`
- **Always run `make generate` to update documentation**
- Update examples in `examples/` if the change affects usage patterns
- Update README.md if adding significant functionality (e.g., import support, new attributes)
- Document any breaking changes or migrations needed

**Testing against local Forgejo:**

- Start with: `docker-compose -f docker/docker-compose.yml up`
- Run acceptance tests with: `make testacc`
