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
	Access                  types.Object `tfsdk:"access"`
	CanCreateOrgRepo        types.Bool   `tfsdk:"can_create_org_repo"`
	IncludesAllRepositories types.Bool   `tfsdk:"includes_all_repositories"`
}

// accessModel represents the access block
type accessModel struct {
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

	// Note: The Forgejo API calculates the permission field as the minimum of all unit permissions
	// in units_map. However, we preserve the permission value from the plan/config since it's a
	// user-configured value, and the actual granular permissions are controlled by the access block
	// (units_map). The returned permission from the API is merely informational.

	if !m.CanCreateOrgRepo.IsNull() {
		m.CanCreateOrgRepo = types.BoolValue(t.CanCreateOrgRepo)
	}
	if !m.IncludesAllRepositories.IsNull() {
		m.IncludesAllRepositories = types.BoolValue(t.IncludesAllRepositories)
	}

	// Note: We do NOT read back the access block values from the API response.
	// The Forgejo API may override unit permissions based on the permission field
	// (e.g., if permission="admin", all units become "admin"). However, we want to preserve
	// the user's explicit configuration, so we keep the access values from the plan/state.
}

// accessAttrTypes returns the attribute types for the accessModel
func accessAttrTypes() map[string]attr.Type {
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

// buildUnitsMap converts the access block to a units_map for the API
func (m *teamResourceModel) buildUnitsMap() map[string]string {
	permission := m.Permission.ValueString()
	allUnits := []string{"repo.code", "repo.issues", "repo.pulls", "repo.ext_issues", "repo.wiki", "repo.ext_wiki", "repo.releases", "repo.projects", "repo.packages", "repo.actions"}

	unitsMap := make(map[string]string)

	// If access block is not set, set all units to the permission level
	// This is required by the Forgejo API (either units or units_map must be specified)
	if m.Access.IsNull() || m.Access.IsUnknown() {
		for _, unit := range allUnits {
			unitsMap[unit] = permission
		}
		return unitsMap
	}

	// Extract access block values
	var access accessModel
	d := m.Access.As(context.Background(), &access, basetypes.ObjectAsOptions{})
	if d.HasError() {
		// If we can't extract, default all units to permission level
		for _, unit := range allUnits {
			unitsMap[unit] = permission
		}
		return unitsMap
	}

	// Track which units were explicitly configured
	configured := make(map[string]bool)

	// Add units that were explicitly set
	if !access.Code.IsNull() {
		unitsMap["repo.code"] = access.Code.ValueString()
		configured["repo.code"] = true
	}
	if !access.Issues.IsNull() {
		unitsMap["repo.issues"] = access.Issues.ValueString()
		configured["repo.issues"] = true
	}
	if !access.Pulls.IsNull() {
		unitsMap["repo.pulls"] = access.Pulls.ValueString()
		configured["repo.pulls"] = true
	}
	if !access.ExtIssues.IsNull() {
		unitsMap["repo.ext_issues"] = access.ExtIssues.ValueString()
		configured["repo.ext_issues"] = true
	}
	if !access.Wiki.IsNull() {
		unitsMap["repo.wiki"] = access.Wiki.ValueString()
		configured["repo.wiki"] = true
	}
	if !access.ExtWiki.IsNull() {
		unitsMap["repo.ext_wiki"] = access.ExtWiki.ValueString()
		configured["repo.ext_wiki"] = true
	}
	if !access.Releases.IsNull() {
		unitsMap["repo.releases"] = access.Releases.ValueString()
		configured["repo.releases"] = true
	}
	if !access.Projects.IsNull() {
		unitsMap["repo.projects"] = access.Projects.ValueString()
		configured["repo.projects"] = true
	}
	if !access.Packages.IsNull() {
		unitsMap["repo.packages"] = access.Packages.ValueString()
		configured["repo.packages"] = true
	}
	if !access.Actions.IsNull() {
		unitsMap["repo.actions"] = access.Actions.ValueString()
		configured["repo.actions"] = true
	}

	// For units not explicitly configured, use the permission level as fallback
	for _, unit := range allUnits {
		if !configured[unit] {
			unitsMap[unit] = permission
		}
	}

	return unitsMap
}

func (m *teamResourceModel) to(o *forgejo.CreateTeamOption) {
	if o == nil {
		o = new(forgejo.CreateTeamOption)
	}

	o.Name = m.Name.ValueString()
	o.Description = m.Description.ValueString()
	o.CanCreateOrgRepo = m.CanCreateOrgRepo.ValueBool()
	o.IncludesAllRepositories = m.IncludesAllRepositories.ValueBool()

	// Set permission level (required)
	o.Permission = forgejo.AccessMode(m.Permission.ValueString())

	// Convert access block to units_map
	// The API's behavior: if units_map is set, permission gets overridden.
	// To avoid this, we use units_map only when access block is explicitly set.
	// When access block is not set, we rely on the permission field.
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

	// Set permission level (required)
	o.Permission = forgejo.AccessMode(m.Permission.ValueString())

	// Convert access block to units_map
	// Always build a full units_map with all units explicitly set
	o.UnitsMap = m.buildUnitsMap()
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
				Description: "Permission level of the team. Possible values are 'read', 'write', or 'admin'.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						"read",
						"write",
						"admin",
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
		"access": schema.SingleNestedBlock{
			Description: "Repository access levels for the team. Each key represents a repository unit, with values specifying the access level ('none', 'read', 'write', 'admin'). Omitted keys default to 'none'.",
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

	tflog.Info(ctx, "Create team", map[string]any{
		"organization":              data.Organization.ValueString(),
		"name":                      data.Name.ValueString(),
		"description":               data.Description.ValueString(),
		"permission":                data.Permission.ValueString(),
		"access":                    data.Access.String(),
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

	// Read Terraform plan data into model
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Update team", map[string]any{
		"id":                        data.ID.ValueInt64(),
		"name":                      data.Name.ValueString(),
		"description":               data.Description.ValueString(),
		"permission":                data.Permission.ValueString(),
		"access":                    data.Access.String(),
		"can_create_org_repo":       data.CanCreateOrgRepo.ValueBool(),
		"includes_all_repositories": data.IncludesAllRepositories.ValueBool(),
	})

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
