package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &accessTokenResource{}
	_ resource.ResourceWithConfigure = &accessTokenResource{}
)

// accessTokenResource is the resource implementation.
type accessTokenResource struct {
	client *forgejo.Client
}

// accessTokenResourceModel maps the resource schema data.
type accessTokenResourceModel struct {
	ID       types.Int64  `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Scopes   types.List   `tfsdk:"scopes"`
	Token    types.String `tfsdk:"token"`
	Username types.String `tfsdk:"username"`
}

// Metadata returns the resource type name.
func (r *accessTokenResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_token"
}

// Schema defines the schema for the resource.
func (r *accessTokenResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Forgejo access token resource. Manages personal access tokens for API authentication.",

		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "Numeric identifier of the access token.",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "Name of the access token.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"scopes": schema.ListAttribute{
				Description: "List of scopes for the access token (e.g., 'repo', 'admin:org_hook').",
				ElementType: types.StringType,
				Optional:    true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"token": schema.StringAttribute{
				Description: "The actual access token value. This is only available when the token is first created and cannot be retrieved later.",
				Computed:    true,
				Sensitive:   true,
			},
			"username": schema.StringAttribute{
				Description: "Username of the user to create the token for. This explicitly specifies which user owns the token.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *accessTokenResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *accessTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer un(trace(ctx, "Create access token resource"))

	var data accessTokenResourceModel

	// Read Terraform plan data into model
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Create access token", map[string]any{
		"name": data.Name.ValueString(),
	})

	// Convert scopes from Terraform types to SDK types
	var scopesList []types.String
	diags = data.Scopes.ElementsAs(ctx, &scopesList, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	scopes := make([]forgejo.AccessTokenScope, len(scopesList))
	for i, s := range scopesList {
		scopes[i] = forgejo.AccessTokenScope(s.ValueString())
	}

	// Build create option - username is required so we always have it
	username := data.Username.ValueString()
	opt := forgejo.CreateAccessTokenOptionWithUsername{
		Name:     data.Name.ValueString(),
		Scopes:   scopes,
		Username: &username,
	}

	// Use Forgejo client to create access token
	token, res, err := r.client.CreateAccessToken(opt)
	if err != nil {
		tflog.Error(ctx, "Error", map[string]any{
			"status": res.Status,
		})

		var msg string
		switch res.StatusCode {
		case 400:
			msg = fmt.Sprintf("Invalid token configuration: %s", err)
		case 403:
			msg = fmt.Sprintf("Forbidden: insufficient permissions to create access token: %s", err)
		case 404:
			msg = fmt.Sprintf("User not found: %s", err)
		case 422:
			msg = fmt.Sprintf("Input validation error: %s", err)
		default:
			msg = fmt.Sprintf("Unknown error: %s", err)
		}
		resp.Diagnostics.AddError("Unable to create access token", msg)

		return
	}

	// Map response to model
	data.from(token)

	// Save data into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *accessTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer un(trace(ctx, "Read access token resource"))

	var data accessTokenResourceModel

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tokenID := data.ID.ValueInt64()

	tflog.Info(ctx, "Get access token", map[string]any{
		"id": tokenID,
	})

	// Username is required, so we always know which user's tokens to list
	username := data.Username.ValueString()
	listOpts := forgejo.ListAccessTokensOptions{
		Username: &username,
	}

	// Use Forgejo client to list tokens for the specified user
	tokens, res, err := r.client.ListAccessTokens(listOpts)
	if err != nil {
		tflog.Error(ctx, "Error", map[string]any{
			"status": res.Status,
		})

		var msg string
		switch res.StatusCode {
		case 401:
			msg = fmt.Sprintf("Unauthorized: authentication required to list access tokens: %s", err)
		case 403:
			msg = fmt.Sprintf("Forbidden: insufficient permissions to list access tokens: %s", err)
		default:
			msg = fmt.Sprintf("Unknown error: %s", err)
		}
		resp.Diagnostics.AddError("Unable to list access tokens", msg)

		return
	}

	// Find token by ID
	var foundToken *forgejo.AccessToken
	for _, t := range tokens {
		if t.ID == tokenID {
			foundToken = t
			break
		}
	}

	if foundToken == nil {
		tflog.Warn(ctx, "Access token not found", map[string]any{
			"id":       tokenID,
			"username": username,
		})
		// Token doesn't exist anymore, remove from state
		resp.State.RemoveResource(ctx)
		return
	}

	// Store the existing token value from state before updating
	existingToken := data.Token

	// Map response to model (note: token value is not available on read)
	data.from(foundToken)

	// Restore token value from state if API returned empty
	// (API only returns token on creation, not on subsequent reads)
	if data.Token.IsNull() || data.Token.ValueString() == "" {
		data.Token = existingToken
	}

	// Save data into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Update is not supported for access tokens - all fields are immutable.
func (r *accessTokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Access tokens are immutable - all changes force replacement
	// This method should never be called due to RequiresReplace() modifiers
	resp.Diagnostics.AddError(
		"Unsupported Operation",
		"Access tokens are immutable. Changes to any field require resource recreation.",
	)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *accessTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	defer un(trace(ctx, "Delete access token resource"))

	var data accessTokenResourceModel

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tokenID := data.ID.ValueInt64()
	username := data.Username.ValueString()

	tflog.Info(ctx, "Delete access token", map[string]any{
		"id":       tokenID,
		"username": username,
	})

	// Build delete options - username is required so we always specify it
	deleteOpts := forgejo.DeleteAccessTokensOptions{
		TokenID:  tokenID,
		Username: &username,
	}

	// Use Forgejo client to delete access token
	res, err := r.client.DeleteAccessToken(deleteOpts)
	if err != nil {
		tflog.Error(ctx, "Error", map[string]any{
			"status": res.Status,
		})

		var msg string
		switch res.StatusCode {
		case 401:
			msg = fmt.Sprintf("Unauthorized: authentication required to delete access token: %s", err)
		case 403:
			msg = fmt.Sprintf("Forbidden: insufficient permissions to delete access token: %s", err)
		case 404:
			msg = fmt.Sprintf("Access token not found: %s", err)
		case 422:
			msg = fmt.Sprintf("Input validation error: %s", err)
		default:
			msg = fmt.Sprintf("Unknown error: %s", err)
		}
		resp.Diagnostics.AddError("Unable to delete access token", msg)

		return
	}
}

// from converts a Forgejo AccessToken to the resource model.
func (m *accessTokenResourceModel) from(t *forgejo.AccessToken) {
	m.ID = types.Int64Value(t.ID)
	m.Name = types.StringValue(t.Name)

	// Convert scopes from API
	// Note: Scopes may be reordered by the API, but we preserve them as-is
	if t.Scopes != nil && len(t.Scopes) > 0 {
		scopeValues := make([]attr.Value, len(t.Scopes))
		for i, scope := range t.Scopes {
			scopeValues[i] = types.StringValue(string(scope))
		}
		m.Scopes = types.ListValueMust(types.StringType, scopeValues)
	} else {
		// No scopes returned
		m.Scopes = types.ListNull(types.StringType)
	}

	// Token value is only available on creation (API returns empty on subsequent reads)
	// Only set if it's not empty
	if t.Token != "" {
		m.Token = types.StringValue(t.Token)
	}
}

// NewAccessTokenResource is a helper function to simplify the provider implementation.
func NewAccessTokenResource() resource.Resource {
	return &accessTokenResource{}
}
