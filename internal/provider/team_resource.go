package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
	Units                   types.List   `tfsdk:"units"`
	CanCreateOrgRepo        types.Bool   `tfsdk:"can_create_org_repo"`
	IncludesAllRepositories types.Bool   `tfsdk:"includes_all_repositories"`
	UnitsMap                types.Map    `tfsdk:"units_map"`
}

func (m *teamResourceModel) from(ctx context.Context, t *forgejo.Team) {
	m.ID = types.Int64Value(t.ID)
	if t.Organization != nil {
		m.Organization = types.StringValue(t.Organization.UserName)
	} else {
		m.Organization = types.StringNull()
	}
	m.Name = types.StringValue(t.Name)
	m.Description = types.StringValue(t.Description)
	m.Permission = types.StringValue(string(t.Permission))
	m.CanCreateOrgRepo = types.BoolValue(t.CanCreateOrgRepo)
	m.IncludesAllRepositories = types.BoolValue(t.IncludesAllRepositories)

	// Convert Units list from API response
	if len(t.Units) > 0 {
		unitStrings := make([]string, len(t.Units))
		for i, unit := range t.Units {
			unitStrings[i] = string(unit)
		}
		unitsValue, _ := types.ListValueFrom(ctx, types.StringType, unitStrings)
		m.Units = unitsValue
	} else {
		m.Units = types.ListNull(types.StringType)
	}

	// Convert UnitsMap from API response
	if len(t.UnitsMap) > 0 {
		unitsMapValue, _ := types.MapValueFrom(ctx, types.StringType, t.UnitsMap)
		m.UnitsMap = unitsMapValue
	} else {
		m.UnitsMap = types.MapNull(types.StringType)
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

	// Set permission level (required)
	o.Permission = forgejo.AccessMode(m.Permission.ValueString())

	// Extract units list
	if !m.Units.IsNull() && !m.Units.IsUnknown() {
		var unitStrings []string
		_ = m.Units.ElementsAs(context.Background(), &unitStrings, false)
		for _, unit := range unitStrings {
			o.Units = append(o.Units, forgejo.RepoUnitType(unit))
		}
	}

	// Extract units_map
	if !m.UnitsMap.IsNull() && !m.UnitsMap.IsUnknown() {
		unitsMapValue := make(map[string]string)
		_ = m.UnitsMap.ElementsAs(context.Background(), &unitsMapValue, false)
		o.UnitsMap = unitsMapValue
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

	// Set permission level (required)
	o.Permission = forgejo.AccessMode(m.Permission.ValueString())

	// Extract units list
	if !m.Units.IsNull() && !m.Units.IsUnknown() {
		var unitStrings []string
		_ = m.Units.ElementsAs(context.Background(), &unitStrings, false)
		for _, unit := range unitStrings {
			o.Units = append(o.Units, forgejo.RepoUnitType(unit))
		}
	}

	// Extract units_map
	if !m.UnitsMap.IsNull() && !m.UnitsMap.IsUnknown() {
		unitsMapValue := make(map[string]string)
		_ = m.UnitsMap.ElementsAs(context.Background(), &unitsMapValue, false)
		o.UnitsMap = unitsMapValue
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
			"units": schema.ListAttribute{
				Description: "Repository unit types the team has access to (e.g., 'repo.code', 'repo.issues', 'repo.pulls'). Can be omitted when using units_map.",
				ElementType: types.StringType,
				Optional:    true,
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
				Description: "A map of repository unit types to access modes for this team.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
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
		"units":                     data.Units.String(),
		"can_create_org_repo":       data.CanCreateOrgRepo.ValueBool(),
		"includes_all_repositories": data.IncludesAllRepositories.ValueBool(),
		"units_map":                 data.UnitsMap.String(),
	})

	// Generate API request body from plan
	opts := forgejo.CreateTeamOption{}
	data.to(&opts)

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
		"units":                     data.Units.String(),
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
