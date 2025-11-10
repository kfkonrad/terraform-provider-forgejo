package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &teamResource{}
	_ resource.ResourceWithConfigure = &teamResource{}
	_ resource.ResourceWithValidateConfig = &teamResource{}
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
	Permissions             *permissionsModel `tfsdk:"permissions"`
	CanCreateOrgRepo        types.Bool   `tfsdk:"can_create_org_repo"`
	IncludesAllRepositories types.Bool   `tfsdk:"includes_all_repositories"`
	UnitsMap                types.Map    `tfsdk:"units_map"`
}

// permissionsModel represents the nested permissions block
type permissionsModel struct {
	Level types.String `tfsdk:"level"`
	Units types.List   `tfsdk:"units"`
}

func (m *teamResourceModel) from(ctx context.Context, t *forgejo.Team) {
	// Preserve existing values for optional fields to avoid drift
	// Get the current values before updating
	oldDescription := m.Description
	oldCanCreateOrgRepo := m.CanCreateOrgRepo
	oldIncludesAllRepositories := m.IncludesAllRepositories

	m.ID = types.Int64Value(t.ID)
	if t.Organization != nil {
		m.Organization = types.StringValue(t.Organization.UserName)
	} else {
		m.Organization = types.StringNull()
	}
	m.Name = types.StringValue(t.Name)

	// Only update description if it was set in the plan (not null)
	if !oldDescription.IsNull() {
		m.Description = types.StringValue(t.Description)
	}

	// Convert UnitsMap from API response
	if len(t.UnitsMap) > 0 {
		unitsMapValue, _ := types.MapValueFrom(ctx, types.StringType, t.UnitsMap)
		m.UnitsMap = unitsMapValue
	} else {
		m.UnitsMap = types.MapNull(types.StringType)
	}

	// Populate permissions block (single nested block)
	// Simple approach: trust what the user configured
	// If they specified permissions block, preserve it from plan
	// If they didn't, keep it null
	// Only override to null if API returned units_map (which means they used units_map)
	apiHasUnitsMap := len(t.UnitsMap) > 0

	if apiHasUnitsMap {
		// API returned units_map, so user used units_map
		// Don't populate permissions to maintain the config style distinction
		m.Permissions = nil
	}
	// Otherwise, keep m.Permissions as-is from the plan
	// It will be nil if the user didn't specify the permissions block
	// It will have content if they did specify it

	// Only update boolean flags if they were set in the plan (not null)
	if !oldCanCreateOrgRepo.IsNull() {
		m.CanCreateOrgRepo = types.BoolValue(t.CanCreateOrgRepo)
	}
	if !oldIncludesAllRepositories.IsNull() {
		m.IncludesAllRepositories = types.BoolValue(t.IncludesAllRepositories)
	}
}

func (m *teamResourceModel) to(o *forgejo.CreateTeamOption) {
	if o == nil {
		o = new(forgejo.CreateTeamOption)
	}

	o.Name = m.Name.ValueString()
	o.Description = m.Description.ValueString()
	o.CanCreateOrgRepo = m.CanCreateOrgRepo.ValueBool()
	o.IncludesAllRepositories = m.IncludesAllRepositories.ValueBool()

	// Handle UnitsMap vs Permissions - mutually exclusive
	if !m.UnitsMap.IsNull() && !m.UnitsMap.IsUnknown() {
		// Using UnitsMap - extract map and set it
		unitsMapValue := make(map[string]string)
		_ = m.UnitsMap.ElementsAs(context.Background(), &unitsMapValue, false)
		o.UnitsMap = unitsMapValue
		// Default permission when using unitsmap
		o.Permission = forgejo.AccessModeWrite
		// Set default units for API - repo.code is required
		o.Units = []forgejo.RepoUnitType{forgejo.RepoUnitCode}
	} else if m.Permissions != nil {
		// Using Permissions block
		perm := m.Permissions

		// Extract permission level
		permission := perm.Level.ValueString()
		if permission == "" {
			permission = "write"
		}
		o.Permission = forgejo.AccessMode(permission)

		// Extract units if specified
		var units []forgejo.RepoUnitType
		if !perm.Units.IsNull() && !perm.Units.IsUnknown() {
			var unitStrings []string
			_ = perm.Units.ElementsAs(context.Background(), &unitStrings, false)
			for _, unit := range unitStrings {
				units = append(units, forgejo.RepoUnitType(unit))
			}
		}
		if len(units) == 0 {
			// Default to repo.code if no units specified
			units = []forgejo.RepoUnitType{forgejo.RepoUnitCode}
		}
		o.Units = units
	} else {
		// Default behavior if neither is set
		o.Permission = forgejo.AccessModeWrite
		o.Units = []forgejo.RepoUnitType{forgejo.RepoUnitCode}
	}
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

	// Handle UnitsMap vs Permissions - mutually exclusive
	if !m.UnitsMap.IsNull() && !m.UnitsMap.IsUnknown() {
		// Using UnitsMap - extract map and set it
		unitsMapValue := make(map[string]string)
		_ = m.UnitsMap.ElementsAs(context.Background(), &unitsMapValue, false)
		o.UnitsMap = unitsMapValue
		// Default permission when using unitsmap
		permission := forgejo.AccessModeWrite
		o.Permission = permission
		// Set default units for API - repo.code is required
		o.Units = []forgejo.RepoUnitType{forgejo.RepoUnitCode}
	} else if m.Permissions != nil {
		// Using Permissions block
		perm := m.Permissions

		// Extract permission level
		permission := perm.Level.ValueString()
		if permission == "" {
			permission = "write"
		}
		o.Permission = forgejo.AccessMode(permission)

		// Extract units if specified
		var units []forgejo.RepoUnitType
		if !perm.Units.IsNull() && !perm.Units.IsUnknown() {
			var unitStrings []string
			_ = perm.Units.ElementsAs(context.Background(), &unitStrings, false)
			for _, unit := range unitStrings {
				units = append(units, forgejo.RepoUnitType(unit))
			}
		}
		if len(units) == 0 {
			// Default to repo.code if no units specified
			units = []forgejo.RepoUnitType{forgejo.RepoUnitCode}
		}
		o.Units = units
	} else {
		// Default behavior if neither is set
		o.Permission = forgejo.AccessModeWrite
		o.Units = []forgejo.RepoUnitType{forgejo.RepoUnitCode}
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
			"can_create_org_repo": schema.BoolAttribute{
				Description: "Whether the team can create repositories in the organization.",
				Optional:    true,
			},
			"includes_all_repositories": schema.BoolAttribute{
				Description: "Whether the team has access to all repositories in the organization.",
				Optional:    true,
			},
			"units_map": schema.MapAttribute{
				Description: "A map of repository unit types to access modes for this team. Mutually exclusive with permissions block.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"permissions": schema.SingleNestedBlock{
				Description: "Permission configuration for the team. Mutually exclusive with units_map.",
				Attributes: map[string]schema.Attribute{
					"level": schema.StringAttribute{
						Description: "Permission level of the team. Possible values are 'read', 'write' (default), or 'admin'.",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf(
								"read",
								"write",
								"admin",
							),
						},
					},
					"units": schema.ListAttribute{
						Description: "Repository unit types the team has access to (e.g., 'repo.code', 'repo.issues', 'repo.pulls'). If not specified, all units are included.",
						ElementType: types.StringType,
						Optional:    true,
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

// ValidateConfig validates the resource configuration.
func (r *teamResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data teamResourceModel

	// Read Terraform configuration data into model
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate that permissions block and units_map are mutually exclusive
	hasUnitsMap := !data.UnitsMap.IsNull() && !data.UnitsMap.IsUnknown()
	hasPermissions := data.Permissions != nil

	if hasUnitsMap && hasPermissions {
		resp.Diagnostics.Append(diag.NewAttributeErrorDiagnostic(
			path.Root("permissions"),
			"Mutually Exclusive Attributes",
			"permissions block and units_map attribute are mutually exclusive. "+
				"Please use only one of them. Use permissions block for the legacy permission/units approach, "+
				"or use units_map for fine-grained per-unit access control.",
		))
		resp.Diagnostics.Append(diag.NewAttributeErrorDiagnostic(
			path.Root("units_map"),
			"Mutually Exclusive Attributes",
			"permissions block and units_map attribute are mutually exclusive. "+
				"Please use only one of them. Use permissions block for the legacy permission/units approach, "+
				"or use units_map for fine-grained per-unit access control.",
		))
	}
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

	permissionsDesc := "none"
	if data.Permissions != nil {
		permissionsDesc = fmt.Sprintf("level=%s", data.Permissions.Level.ValueString())
	}

	tflog.Info(ctx, "Create team", map[string]any{
		"organization":              data.Organization.ValueString(),
		"name":                      data.Name.ValueString(),
		"description":               data.Description.ValueString(),
		"permissions":               permissionsDesc,
		"can_create_org_repo":       data.CanCreateOrgRepo.ValueBool(),
		"includes_all_repositories": data.IncludesAllRepositories.ValueBool(),
		"units_map":                 data.UnitsMap.String(),
	})

	// Generate API request body from plan
	opts := forgejo.CreateTeamOption{}
	data.to(&opts)

	// Save the original permissions block from the plan to restore later
	// The API response won't distinguish between permissions block and units_map usage
	// so we need to remember what the user configured
	originalPermissions := data.Permissions
	originalUnitsMap := data.UnitsMap

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

	// Restore the original configuration style that the user specified
	// The API doesn't distinguish between permissions block and units_map,
	// but we need to maintain the user's chosen configuration style
	if originalPermissions != nil {
		// User configured permissions block, restore it
		data.Permissions = originalPermissions
	} else if !originalUnitsMap.IsNull() && !originalUnitsMap.IsUnknown() {
		// User configured units_map, make sure permissions is null
		data.Permissions = nil
	}

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

	permissionsDesc := "none"
	if data.Permissions != nil {
		permissionsDesc = fmt.Sprintf("level=%s", data.Permissions.Level.ValueString())
	}

	tflog.Info(ctx, "Update team", map[string]any{
		"id":                        data.ID.ValueInt64(),
		"name":                      data.Name.ValueString(),
		"description":               data.Description.ValueString(),
		"permissions":               permissionsDesc,
		"can_create_org_repo":       data.CanCreateOrgRepo.ValueBool(),
		"includes_all_repositories": data.IncludesAllRepositories.ValueBool(),
		"units_map":                 data.UnitsMap.String(),
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
	// For updates, read the prior state first to preserve configuration style
	var priorData teamResourceModel
	priorDiags := req.State.Get(ctx, &priorData)
	resp.Diagnostics.Append(priorDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.from(ctx, team)

	// Preserve the configuration style from the prior state
	// This ensures we don't switch between permissions and units_map representations
	if priorData.Permissions != nil {
		// Prior state had permissions, preserve it
		data.Permissions = priorData.Permissions
	} else if !priorData.UnitsMap.IsNull() && !priorData.UnitsMap.IsUnknown() {
		// Prior state had units_map, make sure permissions is null
		data.Permissions = nil
	}

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
