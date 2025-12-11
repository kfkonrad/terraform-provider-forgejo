package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &teamMembershipResource{}
	_ resource.ResourceWithConfigure   = &teamMembershipResource{}
	_ resource.ResourceWithImportState = &teamMembershipResource{}
)

// teamMembershipResource is the resource implementation.
type teamMembershipResource struct {
	client *forgejo.Client
}

// teamMembershipResourceModel maps the resource schema data.
type teamMembershipResourceModel struct {
	TeamID   types.Int64  `tfsdk:"team_id"`
	Username types.String `tfsdk:"username"`
}

// Metadata returns the resource type name.
func (r *teamMembershipResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_membership"
}

// Schema defines the schema for the resource.
func (r *teamMembershipResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Forgejo team membership resource. Manages membership of users in teams.",

		Attributes: map[string]schema.Attribute{
			"team_id": schema.Int64Attribute{
				Description: "Numeric identifier of the team.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"username": schema.StringAttribute{
				Description: "Username of the user to add to the team.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *teamMembershipResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *teamMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer un(trace(ctx, "Create team membership resource"))

	var data teamMembershipResourceModel

	// Read Terraform plan data into model
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	teamID := data.TeamID.ValueInt64()
	username := data.Username.ValueString()

	tflog.Info(ctx, "Add user to team", map[string]any{
		"team_id":  teamID,
		"username": username,
	})

	// Use Forgejo client to add user to team
	res, err := r.client.AddTeamMember(teamID, username)
	if err != nil {
		tflog.Error(ctx, "Error", map[string]any{
			"status": res.Status,
		})

		var msg string
		switch res.StatusCode {
		case 403:
			msg = fmt.Sprintf(
				"User %s cannot be added to team %d (forbidden): %s",
				username,
				teamID,
				err,
			)
		case 404:
			msg = fmt.Sprintf(
				"Team with id %d or user %s not found: %s",
				teamID,
				username,
				err,
			)
		case 422:
			msg = fmt.Sprintf("Input validation error: %s", err)
		default:
			msg = fmt.Sprintf("Unknown error: %s", err)
		}
		resp.Diagnostics.AddError("Unable to add user to team", msg)

		return
	}

	// Save data into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *teamMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer un(trace(ctx, "Read team membership resource"))

	var data teamMembershipResourceModel

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	teamID := data.TeamID.ValueInt64()
	username := data.Username.ValueString()

	tflog.Info(ctx, "Get team member", map[string]any{
		"team_id":  teamID,
		"username": username,
	})

	// Use Forgejo client to get team member
	_, res, err := r.client.GetTeamMember(teamID, username)
	if err != nil {
		tflog.Error(ctx, "Error", map[string]any{
			"status": res.Status,
		})

		if res.StatusCode == 404 {
			// Resource doesn't exist anymore, remove from state
			resp.State.RemoveResource(ctx)
			return
		}

		msg := fmt.Sprintf("Unknown error: %s", err)
		resp.Diagnostics.AddError("Unable to get team member", msg)

		return
	}

	// Save data into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Update is not supported for team membership - the resource is immutable.
// Changing team_id or username requires recreation.
func (r *teamMembershipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Team membership is immutable - all changes force replacement
	// This method should never be called due to RequiresReplace() modifiers
	resp.Diagnostics.AddError(
		"Unsupported Operation",
		"Team membership cannot be updated. Changes to team_id or username require resource recreation.",
	)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *teamMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	defer un(trace(ctx, "Delete team membership resource"))

	var data teamMembershipResourceModel

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	teamID := data.TeamID.ValueInt64()
	username := data.Username.ValueString()

	tflog.Info(ctx, "Remove user from team", map[string]any{
		"team_id":  teamID,
		"username": username,
	})

	// Use Forgejo client to remove user from team
	res, err := r.client.RemoveTeamMember(teamID, username)
	if err != nil {
		tflog.Error(ctx, "Error", map[string]any{
			"status": res.Status,
		})

		var msg string
		switch res.StatusCode {
		case 403:
			msg = fmt.Sprintf(
				"User %s cannot be removed from team %d (forbidden): %s",
				username,
				teamID,
				err,
			)
		case 404:
			msg = fmt.Sprintf(
				"Team member not found - team id: %d, username: %s: %s",
				teamID,
				username,
				err,
			)
		case 422:
			msg = fmt.Sprintf("Input validation error: %s", err)
		default:
			msg = fmt.Sprintf("Unknown error: %s", err)
		}
		resp.Diagnostics.AddError("Unable to remove user from team", msg)

		return
	}
}

// ImportState imports the resource state from team_id:username format.
func (r *teamMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	defer un(trace(ctx, "Import team membership resource"))

	// ID format: team_id:username
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected format: team_id:username, got: %s", req.ID),
		)
		return
	}

	teamIDStr := parts[0]
	username := parts[1]

	// Parse team ID
	teamID, err := strconv.ParseInt(teamIDStr, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid team ID",
			fmt.Sprintf("Team ID must be numeric, got: %s", teamIDStr),
		)
		return
	}

	tflog.Info(ctx, "Importing team membership", map[string]any{
		"team_id":  teamID,
		"username": username,
	})

	// Verify the team member exists
	_, res, err := r.client.GetTeamMember(teamID, username)
	if err != nil {
		var msg string
		if res != nil {
			switch res.StatusCode {
			case 404:
				msg = fmt.Sprintf(
					"Team member not found - team id: %d, username: %s: %s",
					teamID,
					username,
					err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		} else {
			msg = fmt.Sprintf("Unknown error: %s", err)
		}
		resp.Diagnostics.AddError("Unable to verify team member", msg)
		return
	}

	data := teamMembershipResourceModel{
		TeamID:   types.Int64Value(teamID),
		Username: types.StringValue(username),
	}

	// Save data into Terraform state
	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)

	tflog.Info(ctx, "Team membership imported successfully", map[string]any{
		"team_id":  teamID,
		"username": username,
	})
}

// NewTeamMembershipResource is a helper function to simplify the provider implementation.
func NewTeamMembershipResource() resource.Resource {
	return &teamMembershipResource{}
}
