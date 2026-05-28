package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &teamResource{}
	_ resource.ResourceWithConfigure   = &teamResource{}
	_ resource.ResourceWithImportState = &teamResource{}
)

// teamResource is the resource implementation.
type teamResource struct {
	client *forgejo.Client
}

// teamResourceModel maps the resource schema data.
// https://pkg.go.dev/codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3#Team
type teamResourceModel struct {
	ID                      types.Int64  `tfsdk:"id"`
	Organization            types.String `tfsdk:"organization"`
	Name                    types.String `tfsdk:"name"`
	Description             types.String `tfsdk:"description"`
	IsAdmin                 types.Bool   `tfsdk:"is_admin"`
	Permissions             types.Object `tfsdk:"permissions"`
	CanCreateOrgRepo        types.Bool   `tfsdk:"can_create_org_repo"`
	IncludesAllRepositories types.Bool   `tfsdk:"includes_all_repositories"`
}

// permissionsModel represents the permissions block.
type permissionsModel struct {
	Code      types.String `tfsdk:"code"`
	Issues    types.String `tfsdk:"issues"`
	Pulls     types.String `tfsdk:"pulls"`
	ExtIssues types.String `tfsdk:"ext_issues"`
	Wiki      types.String `tfsdk:"wiki"`
	ExtWiki   types.String `tfsdk:"ext_wiki"`
	Releases  types.String `tfsdk:"releases"`
	Projects  types.String `tfsdk:"projects"`
	Packages  types.String `tfsdk:"packages"`
	Actions   types.String `tfsdk:"actions"`
}

func (m *teamResourceModel) from(ctx context.Context, t *forgejo.Team) {
	m.ID = types.Int64Value(t.ID)
	if t.Organization != nil {
		m.Organization = types.StringValue(t.Organization.UserName)
	} else {
		m.Organization = types.StringNull()
	}
	m.Name = types.StringValue(t.Name)

	// Only update optional fields if they are already set in the state
	// This prevents state inconsistency when optional fields are omitted from config
	if !m.Description.IsNull() {
		m.Description = types.StringValue(t.Description)
	}

	// Handle permissions based on is_admin setting:
	// - If is_admin = true: Leave permissions block alone (it's ignored by the API)
	// - If is_admin = false: Read API values and convert, downgrading admin to read
	if !m.IsAdmin.IsNull() {
		if !m.IsAdmin.ValueBool() {
			// Granular mode - read from API and map values
			m.readPermissionsFromAPI(ctx, t)
		}
		// Admin mode - don't touch permissions block, let it be ignored
	}

	if !m.CanCreateOrgRepo.IsNull() {
		m.CanCreateOrgRepo = types.BoolValue(t.CanCreateOrgRepo)
	}
	if !m.IncludesAllRepositories.IsNull() {
		m.IncludesAllRepositories = types.BoolValue(t.IncludesAllRepositories)
	}
}

// readPermissionsFromAPI reads the units_map from API and converts to permissions block.
func (m *teamResourceModel) readPermissionsFromAPI(ctx context.Context, t *forgejo.Team) {
	if len(t.UnitsMap) == 0 {
		// No units from API, keep existing permissions (don't change it)
		return
	}

	// Extract current permissions to check which fields were explicitly set by user
	var currentPerms permissionsModel
	if !m.Permissions.IsNull() && !m.Permissions.IsUnknown() {
		d := m.Permissions.As(ctx, &currentPerms, basetypes.ObjectAsOptions{})
		if d.HasError() {
			// Can't extract, skip
			return
		}
	}

	// Build new permissions from API values
	// Strategy: For fields that were set by user, sync from API (converting admin->read)
	//           For fields that were NOT set by user (null), keep them null
	newPerms := permissionsModel{}

	// Define a helper to handle each field
	handleField := func(apiKey string, currentField types.String) types.String {
		apiValue := t.UnitsMap[apiKey]

		// If field was null in current state, keep it null
		if currentField.IsNull() {
			return types.StringNull()
		}

		// Field was set by user, so sync from API
		switch apiValue {
		case "":
			// API doesn't have a value, this shouldn't happen but preserve current
			return currentField
		case "admin":
			// Downgrade admin to read in granular mode
			return types.StringValue("read")
		default:
			// Use API value as-is (read, write, none)
			return types.StringValue(apiValue)
		}
	}

	newPerms.Code = handleField("repo.code", currentPerms.Code)
	newPerms.Issues = handleField("repo.issues", currentPerms.Issues)
	newPerms.Pulls = handleField("repo.pulls", currentPerms.Pulls)
	newPerms.ExtIssues = handleField("repo.ext_issues", currentPerms.ExtIssues)
	newPerms.Wiki = handleField("repo.wiki", currentPerms.Wiki)
	newPerms.ExtWiki = handleField("repo.ext_wiki", currentPerms.ExtWiki)
	newPerms.Releases = handleField("repo.releases", currentPerms.Releases)
	newPerms.Projects = handleField("repo.projects", currentPerms.Projects)
	newPerms.Packages = handleField("repo.packages", currentPerms.Packages)
	newPerms.Actions = handleField("repo.actions", currentPerms.Actions)

	// Convert model to object
	newPermsObj, d := types.ObjectValueFrom(ctx, permissionsAttrTypes(), newPerms)
	if d.HasError() {
		// Can't convert, keep existing
		return
	}
	m.Permissions = newPermsObj
}

// permissionsAttrTypes returns the attribute types for the permissionsModel.
func permissionsAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"code":       types.StringType,
		"issues":     types.StringType,
		"pulls":      types.StringType,
		"ext_issues": types.StringType,
		"wiki":       types.StringType,
		"ext_wiki":   types.StringType,
		"releases":   types.StringType,
		"projects":   types.StringType,
		"packages":   types.StringType,
		"actions":    types.StringType,
	}
}

// buildUnitsMap converts the permissions block to a units_map for the API
// Behavior depends on is_admin setting:
// - If is_admin=true: all units set to "admin"
// - If is_admin=false: use values from permissions block, defaulting unset to "none".
func (m *teamResourceModel) buildUnitsMap() map[string]string {
	unitsMap := make(map[string]string)

	if m.IsAdmin.ValueBool() {
		// Admin: set all units to admin
		unitsMap["repo.code"] = "admin"
		unitsMap["repo.issues"] = "admin"
		unitsMap["repo.pulls"] = "admin"
		unitsMap["repo.ext_issues"] = "admin"
		unitsMap["repo.wiki"] = "admin"
		unitsMap["repo.ext_wiki"] = "admin"
		unitsMap["repo.releases"] = "admin"
		unitsMap["repo.projects"] = "admin"
		unitsMap["repo.packages"] = "admin"
		unitsMap["repo.actions"] = "admin"
		return unitsMap
	}

	// Granular permissions: extract from block and default unset to "none"
	if m.Permissions.IsNull() || m.Permissions.IsUnknown() {
		// No permissions block, default all to "none"
		unitsMap["repo.code"] = "none"
		unitsMap["repo.issues"] = "none"
		unitsMap["repo.pulls"] = "none"
		unitsMap["repo.ext_issues"] = "none"
		unitsMap["repo.wiki"] = "none"
		unitsMap["repo.ext_wiki"] = "none"
		unitsMap["repo.releases"] = "none"
		unitsMap["repo.projects"] = "none"
		unitsMap["repo.packages"] = "none"
		unitsMap["repo.actions"] = "none"
		return unitsMap
	}

	// Extract permissions block values
	var perms permissionsModel
	d := m.Permissions.As(context.Background(), &perms, basetypes.ObjectAsOptions{})
	if d.HasError() {
		// If we can't extract, default all to "none"
		unitsMap["repo.code"] = "none"
		unitsMap["repo.issues"] = "none"
		unitsMap["repo.pulls"] = "none"
		unitsMap["repo.ext_issues"] = "none"
		unitsMap["repo.wiki"] = "none"
		unitsMap["repo.ext_wiki"] = "none"
		unitsMap["repo.releases"] = "none"
		unitsMap["repo.projects"] = "none"
		unitsMap["repo.packages"] = "none"
		unitsMap["repo.actions"] = "none"
		return unitsMap
	}

	// Build units from permissions, defaulting null/unset to "none"
	unitsMap["repo.code"] = permissionsValueOrDefault(perms.Code)
	unitsMap["repo.issues"] = permissionsValueOrDefault(perms.Issues)
	unitsMap["repo.pulls"] = permissionsValueOrDefault(perms.Pulls)
	unitsMap["repo.ext_issues"] = permissionsValueOrDefault(perms.ExtIssues)
	unitsMap["repo.wiki"] = permissionsValueOrDefault(perms.Wiki)
	unitsMap["repo.ext_wiki"] = permissionsValueOrDefault(perms.ExtWiki)
	unitsMap["repo.releases"] = permissionsValueOrDefault(perms.Releases)
	unitsMap["repo.projects"] = permissionsValueOrDefault(perms.Projects)
	unitsMap["repo.packages"] = permissionsValueOrDefault(perms.Packages)
	unitsMap["repo.actions"] = permissionsValueOrDefault(perms.Actions)

	return unitsMap
}

// permissionsValueOrDefault returns the string value of a types.String, or "none" if null/unknown.
func permissionsValueOrDefault(val types.String) string {
	if val.IsNull() || val.IsUnknown() {
		return "none"
	}
	return val.ValueString()
}

func (m *teamResourceModel) to(o *forgejo.CreateTeamOption) {
	if o == nil {
		o = new(forgejo.CreateTeamOption)
	}

	o.Name = m.Name.ValueString()
	o.Description = m.Description.ValueString()
	o.CanCreateOrgRepo = m.CanCreateOrgRepo.ValueBool()
	o.IncludesAllRepositories = m.IncludesAllRepositories.ValueBool()

	// Permission field handling based on is_admin:
	// - is_admin=true: send "admin" to API, send all units as "admin"
	// - is_admin=false: send "read" to API (default), send granular units instead
	if !m.IsAdmin.IsNull() {
		if m.IsAdmin.ValueBool() {
			o.Permission = forgejo.AccessMode("admin")
		} else {
			// For granular, send "read" as the base permission level
			o.Permission = forgejo.AccessMode("read")
		}
	}

	// Always build and send units_map based on is_admin setting
	o.UnitsMap = m.buildUnitsMap()
}

func (m *teamResourceModel) toEdit(o *forgejo.EditTeamOption) {
	if o == nil {
		o = new(forgejo.EditTeamOption)
	}

	o.Name = m.Name.ValueString()
	description := m.Description.ValueString()
	o.Description = &description
	canCreateOrgRepo := m.CanCreateOrgRepo.ValueBool()
	o.CanCreateOrgRepo = &canCreateOrgRepo
	includesAllRepositories := m.IncludesAllRepositories.ValueBool()
	o.IncludesAllRepositories = &includesAllRepositories

	// Permission field handling based on is_admin:
	// - is_admin=true: send "admin" to API, send all units as "admin"
	// - is_admin=false: send "read" to API (default), send granular units instead
	if !m.IsAdmin.IsNull() {
		if m.IsAdmin.ValueBool() {
			o.Permission = forgejo.AccessMode("admin")
		} else {
			// For granular, send "read" as the base permission level
			o.Permission = forgejo.AccessMode("read")
		}
	}

	// Always build and send units_map based on is_admin setting
	o.UnitsMap = m.buildUnitsMap()
}

// validateIsAdmin checks that permissions block is set when is_admin is false.
func (m *teamResourceModel) validateIsAdmin() string {
	if m.IsAdmin.IsNull() || m.IsAdmin.IsUnknown() {
		return "" // is_admin is required, will be caught by schema validation
	}

	// Only require permissions block when is_admin = false
	if !m.IsAdmin.ValueBool() && (m.Permissions.IsNull() || m.Permissions.IsUnknown()) {
		return "permissions must be set when is_admin is false"
	}

	return ""
}

// permissionValueOrNull converts an API permission value to a terraform type.String
// "none" values are converted to null, "admin" is downgraded to "read" in granular mode.
func permissionValueOrNull(apiValue string) types.String {
	if apiValue == "" || apiValue == "none" {
		return types.StringNull()
	}
	if apiValue == "admin" {
		return types.StringValue("read")
	}
	return types.StringValue(apiValue)
}

// derivePermissionsFromAPI derives is_admin and permissions from API response
// If all units in units_map are "admin", set is_admin=true
// Otherwise, set is_admin=false and build permissions from units_map.
func derivePermissionsFromAPI(m *teamResourceModel, team *forgejo.Team) {
	if len(team.UnitsMap) == 0 {
		// No units from API, default to granular with no specific permissions
		m.IsAdmin = types.BoolValue(false)
		m.Permissions = types.ObjectNull(permissionsAttrTypes())
		return
	}

	// Check if all units are "admin"
	allAdmin := true
	for _, permission := range team.UnitsMap {
		if permission != "admin" {
			allAdmin = false
			break
		}
	}

	if allAdmin {
		// All units are admin, set is_admin=true
		m.IsAdmin = types.BoolValue(true)
		m.Permissions = types.ObjectNull(permissionsAttrTypes())
	} else {
		// Mixed permissions, set is_admin=false and build permissions block
		m.IsAdmin = types.BoolValue(false)

		// Build permissions from units_map
		// Set to null for "none" values, downgrade "admin" to "read"
		perms := permissionsModel{
			Code:      permissionValueOrNull(team.UnitsMap["repo.code"]),
			Issues:    permissionValueOrNull(team.UnitsMap["repo.issues"]),
			Pulls:     permissionValueOrNull(team.UnitsMap["repo.pulls"]),
			ExtIssues: permissionValueOrNull(team.UnitsMap["repo.ext_issues"]),
			Wiki:      permissionValueOrNull(team.UnitsMap["repo.wiki"]),
			ExtWiki:   permissionValueOrNull(team.UnitsMap["repo.ext_wiki"]),
			Releases:  permissionValueOrNull(team.UnitsMap["repo.releases"]),
			Projects:  permissionValueOrNull(team.UnitsMap["repo.projects"]),
			Packages:  permissionValueOrNull(team.UnitsMap["repo.packages"]),
			Actions:   permissionValueOrNull(team.UnitsMap["repo.actions"]),
		}

		// Convert to object type
		ctx := context.Background()
		permsObj, _ := types.ObjectValueFrom(ctx, permissionsAttrTypes(), perms)
		m.Permissions = permsObj
	}
}

// Metadata returns the resource type name.
func (r *teamResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"
}

// Schema defines the schema for the resource.
func (r *teamResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Forgejo team resource.",

		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "Numeric identifier of the team.",
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"organization": schema.StringAttribute{
				Description: "Name of the organization the team belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the team.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 30),
				},
			},
			"description": schema.StringAttribute{
				Description: "Description of the team.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(255),
				},
			},
			"is_admin": schema.BoolAttribute{
				Description: "Whether the team has full administrative access to all repositories. When true, uniform admin access is granted. When false, use the permissions block for fine-grained control.",
				Required:    true,
			},
			"can_create_org_repo": schema.BoolAttribute{
				Description: "Whether the team can create repositories in the organization.",
				Optional:    true,
			},
			"includes_all_repositories": schema.BoolAttribute{
				Description: "Whether the team has access to all repositories in the organization.",
				Optional:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"permissions": schema.SingleNestedBlock{
				Description: "Repository access levels for the team. Required when is_admin=false. Each key represents a repository unit, with values specifying the access level ('none', 'read', 'write', 'admin'). Omitted keys default to 'none'. Ignored when is_admin=true.",
				Attributes: map[string]schema.Attribute{
					"code": schema.StringAttribute{
						Description: "Access level for code (repo.code). Allowed values: `none` (default), `read`, `write`, `admin`.",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("none", "read", "write", "admin"),
						},
					},
					"issues": schema.StringAttribute{
						Description: "Access level for issues (repo.issues). Allowed values: `none` (default), `read`, `write`, `admin`.",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("none", "read", "write", "admin"),
						},
					},
					"pulls": schema.StringAttribute{
						Description: "Access level for pull requests (repo.pulls). Allowed values: `none` (default), `read`, `write`, `admin`.",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("none", "read", "write", "admin"),
						},
					},
					"ext_issues": schema.StringAttribute{
						Description: "Access level for external issues (repo.ext_issues). Allowed values: `none` (default), `read`, `write`, `admin`.",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("none", "read", "write", "admin"),
						},
					},
					"wiki": schema.StringAttribute{
						Description: "Access level for wiki (repo.wiki). Allowed values: `none` (default), `read`, `write`, `admin`.",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("none", "read", "write", "admin"),
						},
					},
					"ext_wiki": schema.StringAttribute{
						Description: "Access level for external wiki (repo.ext_wiki). Allowed values: `none` (default), `read`, `write`, `admin`.",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("none", "read", "write", "admin"),
						},
					},
					"releases": schema.StringAttribute{
						Description: "Access level for releases (repo.releases). Allowed values: `none` (default), `read`, `write`, `admin`.",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("none", "read", "write", "admin"),
						},
					},
					"projects": schema.StringAttribute{
						Description: "Access level for projects (repo.projects). Allowed values: `none` (default), `read`, `write`, `admin`.",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("none", "read", "write", "admin"),
						},
					},
					"packages": schema.StringAttribute{
						Description: "Access level for packages (repo.packages). Allowed values: `none` (default), `read`, `write`, `admin`.",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("none", "read", "write", "admin"),
						},
					},
					"actions": schema.StringAttribute{
						Description: "Access level for actions (repo.actions). Allowed values: `none` (default), `read`, `write`, `admin`.",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("none", "read", "write", "admin"),
						},
					},
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *teamResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*forgejo.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf(
				"Expected *forgejo.Client, got: %T. Please report this issue to the provider developers.",
				req.ProviderData,
			),
		)

		return
	}

	r.client = client
}

// ImportState implements resource.ResourceWithImportState.
// ImportState is called when importing an existing resource.
// The import ID format is: organization:team_name
// Example: terraform import forgejo_team.developers my-org:developers.
func (r *teamResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	defer un(trace(ctx, "Import team resource"))

	// Parse the import ID (format: organization:team_name)
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			fmt.Sprintf("Expected format 'organization:team_name', got: %s", req.ID),
		)
		return
	}

	org := parts[0]
	teamName := parts[1]

	tflog.Info(ctx, "Importing team", map[string]any{
		"organization": org,
		"team_name":    teamName,
	})

	// Search for the team by name in the organization
	teams, res, err := r.client.SearchOrgTeams(org, &forgejo.SearchTeamsOptions{Query: teamName})
	if err != nil {
		if res != nil {
			tflog.Error(ctx, "Error searching teams", map[string]any{
				"status": res.Status,
			})
		}
		resp.Diagnostics.AddError("Unable to import team", fmt.Sprintf("Error searching teams: %s", err))
		return
	}

	// Find exact name match
	var teamID int64
	found := false
	for _, t := range teams {
		if t.Name == teamName {
			teamID = t.ID
			found = true
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError(
			"Unable to import team",
			fmt.Sprintf("No team with name %q found in organization %q", teamName, org),
		)
		return
	}

	// Fetch the full team details from Forgejo API
	team, res, err := r.client.GetTeam(teamID)
	if err != nil {
		if res != nil {
			tflog.Error(ctx, "Error fetching team", map[string]any{
				"status": res.Status,
			})
		}

		var msg string
		if res != nil {
			switch res.StatusCode {
			case 404:
				msg = fmt.Sprintf("Team with ID %d not found", teamID)
			default:
				msg = fmt.Sprintf("Error fetching team: %s", err)
			}
		} else {
			msg = fmt.Sprintf("Error fetching team: %s", err)
		}
		resp.Diagnostics.AddError("Unable to import team", msg)
		return
	}

	// Initialize state model with fetched data
	data := teamResourceModel{
		ID:          types.Int64Value(team.ID),
		Name:        types.StringValue(team.Name),
		Description: types.StringValue(team.Description),
	}

	// Set organization if available
	if team.Organization != nil {
		data.Organization = types.StringValue(team.Organization.UserName)
	}

	// Derive is_admin and permissions from API response
	derivePermissionsFromAPI(&data, team)

	// Set optional fields
	data.CanCreateOrgRepo = types.BoolValue(team.CanCreateOrgRepo)
	data.IncludesAllRepositories = types.BoolValue(team.IncludesAllRepositories)

	// Save the imported state
	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Team imported successfully", map[string]any{
		"id":          team.ID,
		"name":        team.Name,
		"is_admin":    data.IsAdmin.ValueBool(),
		"permissions": data.Permissions.String(),
	})
}

// Create creates the resource and sets the initial Terraform state.
func (r *teamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer un(trace(ctx, "Create team resource"))

	var data teamResourceModel

	// Read Terraform plan data into model
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate permission settings
	if validErr := data.validateIsAdmin(); validErr != "" {
		resp.Diagnostics.AddError("Invalid permission configuration", validErr)
		return
	}

	tflog.Info(ctx, "Create team", map[string]any{
		"organization":              data.Organization.ValueString(),
		"name":                      data.Name.ValueString(),
		"description":               data.Description.ValueString(),
		"is_admin":                  data.IsAdmin.ValueBool(),
		"permissions":               data.Permissions.String(),
		"can_create_org_repo":       data.CanCreateOrgRepo.ValueBool(),
		"includes_all_repositories": data.IncludesAllRepositories.ValueBool(),
	})

	// Generate API request body from plan
	opts := forgejo.CreateTeamOption{}
	data.to(&opts)

	tflog.Info(ctx, "CreateTeamOption to send to API", map[string]any{
		"permission": string(opts.Permission),
		"unitsMap":   opts.UnitsMap,
	})

	// Use Forgejo client to create new team
	team, res, err := r.client.CreateTeam(data.Organization.ValueString(), opts)
	if err != nil {
		if res != nil {
			tflog.Error(ctx, "Error", map[string]any{
				"status": res.Status,
			})
		}

		var msg string
		if res != nil {
			switch res.StatusCode {
			case 403:
				msg = fmt.Sprintf(
					"Not authorized to create team in organization %s: %s",
					data.Organization.String(),
					err,
				)
			case 404:
				msg = fmt.Sprintf(
					"Organization %s not found: %s",
					data.Organization.String(),
					err,
				)
			case 422:
				msg = fmt.Sprintf("Input validation error: %s", err)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		} else {
			msg = fmt.Sprintf("Unknown error: %s", err)
		}
		resp.Diagnostics.AddError("Unable to create team", msg)

		return
	}

	// Map response body to model
	data.from(ctx, team)

	// Save data into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *teamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer un(trace(ctx, "Read team resource"))

	var data teamResourceModel

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Read team", map[string]any{
		"id": data.ID.ValueInt64(),
	})

	// Use Forgejo client to get team by ID
	team, res, err := r.client.GetTeam(data.ID.ValueInt64())
	if err != nil {
		var msg string
		if res == nil {
			msg = fmt.Sprintf("Unknown error with nil response: %s", err)
		} else {
			tflog.Error(ctx, "Error", map[string]any{
				"status": res.Status,
			})

			switch res.StatusCode {
			case 404:
				msg = fmt.Sprintf(
					"Team with id %d not found: %s",
					data.ID.ValueInt64(),
					err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to read team", msg)

		return
	}

	// Map response body to model
	data.from(ctx, team)

	// Save data into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *teamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer un(trace(ctx, "Update team resource"))

	var data teamResourceModel
	var priorData teamResourceModel

	// Read Terraform plan data into model
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read prior state to preserve permission if it's not in the plan
	diags = req.State.Get(ctx, &priorData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Update team", map[string]any{
		"id":                        data.ID.ValueInt64(),
		"name":                      data.Name.ValueString(),
		"description":               data.Description.ValueString(),
		"is_admin":                  data.IsAdmin.ValueBool(),
		"permissions":               data.Permissions.String(),
		"can_create_org_repo":       data.CanCreateOrgRepo.ValueBool(),
		"includes_all_repositories": data.IncludesAllRepositories.ValueBool(),
	})

	// If is_admin is not in the plan, preserve the prior state's is_admin and permissions
	if data.IsAdmin.IsNull() && !priorData.IsAdmin.IsNull() {
		data.IsAdmin = priorData.IsAdmin
	}
	if data.Permissions.IsNull() && !priorData.Permissions.IsNull() && !data.IsAdmin.ValueBool() {
		data.Permissions = priorData.Permissions
	}

	// Validate permission settings
	if validErr := data.validateIsAdmin(); validErr != "" {
		resp.Diagnostics.AddError("Invalid permission configuration", validErr)
		return
	}

	// Generate API request body from plan
	opts := forgejo.EditTeamOption{}
	data.toEdit(&opts)

	// Use Forgejo client to update existing team
	res, err := r.client.EditTeam(data.ID.ValueInt64(), opts)
	if err != nil {
		if res != nil {
			tflog.Error(ctx, "Error", map[string]any{
				"status": res.Status,
			})
		}

		var msg string
		if res != nil {
			switch res.StatusCode {
			case 404:
				msg = fmt.Sprintf(
					"Team with id %d not found: %s",
					data.ID.ValueInt64(),
					err,
				)
			case 422:
				msg = fmt.Sprintf("Input validation error: %s", err)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		} else {
			msg = fmt.Sprintf("Unknown error: %s", err)
		}
		resp.Diagnostics.AddError("Unable to update team", msg)

		return
	}

	// Use Forgejo client to fetch updated team
	team, res, err := r.client.GetTeam(data.ID.ValueInt64())
	if err != nil {
		var msg string
		if res == nil {
			msg = fmt.Sprintf("Unknown error with nil response: %s", err)
		} else {
			tflog.Error(ctx, "Error", map[string]any{
				"status": res.Status,
			})

			switch res.StatusCode {
			case 404:
				msg = fmt.Sprintf(
					"Team with id %d not found: %s",
					data.ID.ValueInt64(),
					err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to read team", msg)

		return
	}

	// Map response body to model
	data.from(ctx, team)

	// Save data into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *teamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	defer un(trace(ctx, "Delete team resource"))

	var data teamResourceModel

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Delete team", map[string]any{
		"id": data.ID.ValueInt64(),
	})

	// Use Forgejo client to delete existing team
	res, err := r.client.DeleteTeam(data.ID.ValueInt64())
	if err != nil {
		var msg string
		if res == nil {
			msg = fmt.Sprintf("Unknown error with nil response: %s", err)
		} else {
			tflog.Error(ctx, "Error", map[string]any{
				"status": res.Status,
			})

			switch res.StatusCode {
			case 404:
				msg = fmt.Sprintf(
					"Team with id %d not found: %s",
					data.ID.ValueInt64(),
					err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to delete team", msg)

		return
	}
}

// NewTeamResource is a helper function to simplify the provider implementation.
func NewTeamResource() resource.Resource {
	return &teamResource{}
}
