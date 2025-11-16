package provider

import (
	"context"
	"fmt"

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

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &teamResource{}
	_ resource.ResourceWithConfigure = &teamResource{}
)

// teamResource is the resource implementation.
type teamResource struct {
	client *forgejo.Client
}

// teamResourceModel maps the resource schema data.
// https://pkg.go.dev/codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2#Team
type teamResourceModel struct {
	ID                      types.Int64  `tfsdk:"id"`
	Organization            types.String `tfsdk:"organization"`
	Name                    types.String `tfsdk:"name"`
	Description             types.String `tfsdk:"description"`
	Permission              types.String `tfsdk:"permission"`
	GranularPermissions     types.Object `tfsdk:"granular_permissions"`
	CanCreateOrgRepo        types.Bool   `tfsdk:"can_create_org_repo"`
	IncludesAllRepositories types.Bool   `tfsdk:"includes_all_repositories"`
}

// granularPermissionsModel represents the granular_permissions block
type granularPermissionsModel struct {
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

	// Handle permission based on what we're configured for:
	// - If state permission is "admin": Keep it as "admin", clear granular_permissions
	// - If state permission is "granular": Read API values and convert, downgrading admin to read
	// Note: The API returns different permission values based on units_map, but we preserve
	// the user's intended permission ("admin" vs "granular") from state
	if !m.Permission.IsNull() {
		switch m.Permission.ValueString() {
		case "admin":
			// Admin permissions - don't read granular_permissions from API
			// Keep permission as "admin" and clear granular_permissions
			m.GranularPermissions = types.ObjectNull(granularPermissionsAttrTypes())
		case "granular":
			// Granular permissions - read from API and map values
			// Even if API returns "read" or other values, we keep state as "granular"
			m.readGranularPermissionsFromAPI(ctx, t)
		}
	}

	if !m.CanCreateOrgRepo.IsNull() {
		m.CanCreateOrgRepo = types.BoolValue(t.CanCreateOrgRepo)
	}
	if !m.IncludesAllRepositories.IsNull() {
		m.IncludesAllRepositories = types.BoolValue(t.IncludesAllRepositories)
	}
}

// readGranularPermissionsFromAPI reads the units_map from API and converts to granular_permissions
func (m *teamResourceModel) readGranularPermissionsFromAPI(ctx context.Context, t *forgejo.Team) {
	if t.UnitsMap == nil || len(t.UnitsMap) == 0 {
		// No units from API, keep existing granular_permissions (don't change it)
		return
	}

	// Extract current granular_permissions to check which fields were explicitly set by user
	var currentGranular granularPermissionsModel
	if !m.GranularPermissions.IsNull() && !m.GranularPermissions.IsUnknown() {
		d := m.GranularPermissions.As(ctx, &currentGranular, basetypes.ObjectAsOptions{})
		if d.HasError() {
			// Can't extract, skip
			return
		}
	}

	// Build new granular_permissions from API values
	// Strategy: For fields that were set by user, sync from API (converting admin->read)
	//           For fields that were NOT set by user (null), keep them null
	newGranular := granularPermissionsModel{}

	// Define a helper to handle each field
	handleField := func(apiKey string, stateField, currentField types.String) types.String {
		apiValue := t.UnitsMap[apiKey]

		// If field was null in current state, keep it null
		if currentField.IsNull() {
			return types.StringNull()
		}

		// Field was set by user, so sync from API
		if apiValue == "" {
			// API doesn't have a value, this shouldn't happen but preserve current
			return currentField
		} else if apiValue == "admin" {
			// Downgrade admin to read in granular mode
			return types.StringValue("read")
		} else {
			// Use API value as-is (read, write, none)
			return types.StringValue(apiValue)
		}
	}

	newGranular.Code = handleField("repo.code", currentGranular.Code, currentGranular.Code)
	newGranular.Issues = handleField("repo.issues", currentGranular.Issues, currentGranular.Issues)
	newGranular.Pulls = handleField("repo.pulls", currentGranular.Pulls, currentGranular.Pulls)
	newGranular.ExtIssues = handleField("repo.ext_issues", currentGranular.ExtIssues, currentGranular.ExtIssues)
	newGranular.Wiki = handleField("repo.wiki", currentGranular.Wiki, currentGranular.Wiki)
	newGranular.ExtWiki = handleField("repo.ext_wiki", currentGranular.ExtWiki, currentGranular.ExtWiki)
	newGranular.Releases = handleField("repo.releases", currentGranular.Releases, currentGranular.Releases)
	newGranular.Projects = handleField("repo.projects", currentGranular.Projects, currentGranular.Projects)
	newGranular.Packages = handleField("repo.packages", currentGranular.Packages, currentGranular.Packages)
	newGranular.Actions = handleField("repo.actions", currentGranular.Actions, currentGranular.Actions)

	// Convert model to object
	newGranularObj, d := types.ObjectValueFrom(ctx, granularPermissionsAttrTypes(), newGranular)
	if d.HasError() {
		// Can't convert, keep existing
		return
	}
	m.GranularPermissions = newGranularObj
}

// granularPermissionsAttrTypes returns the attribute types for the granularPermissionsModel
func granularPermissionsAttrTypes() map[string]attr.Type {
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

// buildUnitsMap converts the granular_permissions block to a units_map for the API
// Behavior depends on permission setting:
// - If permission="admin": all units set to "admin"
// - If permission="granular": use values from granular_permissions block, defaulting unset to "none"
func (m *teamResourceModel) buildUnitsMap() map[string]string {
	unitsMap := make(map[string]string)

	permissionValue := m.Permission.ValueString()

	if permissionValue == "admin" {
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
	if m.GranularPermissions.IsNull() || m.GranularPermissions.IsUnknown() {
		// No granular_permissions block, default all to "none"
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

	// Extract granular_permissions block values
	var granular granularPermissionsModel
	d := m.GranularPermissions.As(context.Background(), &granular, basetypes.ObjectAsOptions{})
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

	// Build units from granular_permissions, defaulting null/unset to "none"
	unitsMap["repo.code"] = granularPermissionsValueOrDefault(granular.Code, "none")
	unitsMap["repo.issues"] = granularPermissionsValueOrDefault(granular.Issues, "none")
	unitsMap["repo.pulls"] = granularPermissionsValueOrDefault(granular.Pulls, "none")
	unitsMap["repo.ext_issues"] = granularPermissionsValueOrDefault(granular.ExtIssues, "none")
	unitsMap["repo.wiki"] = granularPermissionsValueOrDefault(granular.Wiki, "none")
	unitsMap["repo.ext_wiki"] = granularPermissionsValueOrDefault(granular.ExtWiki, "none")
	unitsMap["repo.releases"] = granularPermissionsValueOrDefault(granular.Releases, "none")
	unitsMap["repo.projects"] = granularPermissionsValueOrDefault(granular.Projects, "none")
	unitsMap["repo.packages"] = granularPermissionsValueOrDefault(granular.Packages, "none")
	unitsMap["repo.actions"] = granularPermissionsValueOrDefault(granular.Actions, "none")

	return unitsMap
}

// granularPermissionsValueOrDefault returns the string value of a types.String, or defaultValue if null/unknown
func granularPermissionsValueOrDefault(val types.String, defaultValue string) string {
	if val.IsNull() || val.IsUnknown() {
		return defaultValue
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

	// Permission field handling:
	// - permission="admin": send "admin" to API, send all units as "admin"
	// - permission="granular": send "read" to API (default), send granular units instead
	if !m.Permission.IsNull() {
		permValue := m.Permission.ValueString()
		if permValue == "admin" {
			o.Permission = forgejo.AccessMode("admin")
		} else if permValue == "granular" {
			// For granular, send "read" as the base permission level
			o.Permission = forgejo.AccessMode("read")
		}
	}

	// Always build and send units_map based on permission setting
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

	// Permission field handling:
	// - permission="admin": send "admin" to API, send all units as "admin"
	// - permission="granular": send "read" to API (default), send granular units instead
	if !m.Permission.IsNull() {
		permValue := m.Permission.ValueString()
		if permValue == "admin" {
			o.Permission = forgejo.AccessMode("admin")
		} else if permValue == "granular" {
			// For granular, send "read" as the base permission level
			o.Permission = forgejo.AccessMode("read")
		}
	}

	// Always build and send units_map based on permission setting
	o.UnitsMap = m.buildUnitsMap()
}

// validatePermissions checks that granular_permissions is not set when permission is admin
func (m *teamResourceModel) validatePermissions() string {
	if m.Permission.IsNull() || m.Permission.IsUnknown() {
		return "" // Permission is required, will be caught by schema validation
	}

	permValue := m.Permission.ValueString()
	if permValue == "admin" && !(m.GranularPermissions.IsNull() || m.GranularPermissions.IsUnknown()) {
		return "granular_permissions must not be set when permission is 'admin'"
	}

	if permValue == "granular" && (m.GranularPermissions.IsNull() || m.GranularPermissions.IsUnknown()) {
		return "granular_permissions must be set when permission is 'granular'"
	}

	return ""
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
			"permission": schema.StringAttribute{
				Description: "Permission level of the team. 'admin' grants full administrative access to all repositories. 'granular' allows fine-grained control via the granular_permissions block.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						"admin",
						"granular",
					),
				},
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
			"granular_permissions": schema.SingleNestedBlock{
				Description: "Granular repository access levels for the team. Required when permission='granular'. Each key represents a repository unit, with values specifying the access level ('none', 'read', 'write', 'admin'). Omitted keys default to 'none'. Must not be set when permission='admin'.",
				Attributes: map[string]schema.Attribute{
					"code": schema.StringAttribute{
						Description: "Access level for code (repo.code).",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("none", "read", "write", "admin"),
						},
					},
					"issues": schema.StringAttribute{
						Description: "Access level for issues (repo.issues).",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("none", "read", "write", "admin"),
						},
					},
					"pulls": schema.StringAttribute{
						Description: "Access level for pull requests (repo.pulls).",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("none", "read", "write", "admin"),
						},
					},
					"ext_issues": schema.StringAttribute{
						Description: "Access level for external issues (repo.ext_issues).",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("none", "read", "write", "admin"),
						},
					},
					"wiki": schema.StringAttribute{
						Description: "Access level for wiki (repo.wiki).",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("none", "read", "write", "admin"),
						},
					},
					"ext_wiki": schema.StringAttribute{
						Description: "Access level for external wiki (repo.ext_wiki).",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("none", "read", "write", "admin"),
						},
					},
					"releases": schema.StringAttribute{
						Description: "Access level for releases (repo.releases).",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("none", "read", "write", "admin"),
						},
					},
					"projects": schema.StringAttribute{
						Description: "Access level for projects (repo.projects).",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("none", "read", "write", "admin"),
						},
					},
					"packages": schema.StringAttribute{
						Description: "Access level for packages (repo.packages).",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("none", "read", "write", "admin"),
						},
					},
					"actions": schema.StringAttribute{
						Description: "Access level for actions (repo.actions).",
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
	if validErr := data.validatePermissions(); validErr != "" {
		resp.Diagnostics.AddError("Invalid permission configuration", validErr)
		return
	}

	tflog.Info(ctx, "Create team", map[string]any{
		"organization":              data.Organization.ValueString(),
		"name":                      data.Name.ValueString(),
		"description":               data.Description.ValueString(),
		"permission":                data.Permission.ValueString(),
		"granular_permissions":      data.GranularPermissions.String(),
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

	tflog.Info(ctx, "Get team by id", map[string]any{
		"id": data.ID.ValueInt64(),
	})

	// Use Forgejo client to get team by ID
	team, res, err := r.client.GetTeam(data.ID.ValueInt64())
	if err != nil {
		tflog.Error(ctx, "Error", map[string]any{
			"status": res.Status,
		})

		var msg string
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
		resp.Diagnostics.AddError("Unable to get team by id", msg)

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
		"permission":                data.Permission.ValueString(),
		"granular_permissions":      data.GranularPermissions.String(),
		"can_create_org_repo":       data.CanCreateOrgRepo.ValueBool(),
		"includes_all_repositories": data.IncludesAllRepositories.ValueBool(),
	})

	// If permission is not in the plan, preserve the prior state's permission and granular_permissions
	if data.Permission.IsNull() && !priorData.Permission.IsNull() {
		data.Permission = priorData.Permission
	}
	if data.GranularPermissions.IsNull() && !priorData.GranularPermissions.IsNull() && data.Permission != types.StringValue("admin") {
		data.GranularPermissions = priorData.GranularPermissions
	}

	// Validate permission settings
	if validErr := data.validatePermissions(); validErr != "" {
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
		tflog.Error(ctx, "Error", map[string]any{
			"status": res.Status,
		})

		var msg string
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
		resp.Diagnostics.AddError("Unable to get team by id", msg)

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
		tflog.Error(ctx, "Error", map[string]any{
			"status": res.Status,
		})

		var msg string
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
		resp.Diagnostics.AddError("Unable to delete team", msg)

		return
	}
}

// NewTeamResource is a helper function to simplify the provider implementation.
func NewTeamResource() resource.Resource {
	return &teamResource{}
}
