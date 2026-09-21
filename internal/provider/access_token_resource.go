package provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &accessTokenResource{}
	_ resource.ResourceWithConfigure = &accessTokenResource{}
)

// accessTokenResource is the resource implementation.
type accessTokenResource struct {
	client *forgejo.Client
	api    *apiClient
}

// accessTokenResourceModel maps the resource schema data.
type accessTokenResourceModel struct {
	ID           types.Int64  `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Scopes       types.Set    `tfsdk:"scopes"`
	Repositories types.Set    `tfsdk:"repositories"`
	Token        types.String `tfsdk:"token"`
	Username     types.String `tfsdk:"username"`
}

// accessToken mirrors the API's AccessToken including the `repositories`
// field, which the SDK does not model yet.
type accessToken struct {
	ID           int64            `json:"id"`
	Name         string           `json:"name"`
	Token        string           `json:"sha1"`
	Scopes       []string         `json:"scopes"`
	Repositories []repositoryMeta `json:"repositories"`
}

type repositoryMeta struct {
	ID       int64  `json:"id"`
	Owner    string `json:"owner"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
}

// repoTarget is the API's RepoTargetOption.
type repoTarget struct {
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

// createAccessTokenOption mirrors the API's CreateAccessTokenOption.
type createAccessTokenOption struct {
	Name         string       `json:"name"`
	Scopes       []string     `json:"scopes"`
	Repositories []repoTarget `json:"repositories,omitempty"`
}

var repoFullNameRegexp = regexp.MustCompile(`^[^/\s]+/[^/\s]+$`)

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
			"scopes": schema.SetAttribute{
				Description: "Set of scopes for the access token. Allowed values:\n" +
					"  - `all`\n" +
					"  - `public-only`\n" +
					"  - `sudo`\n" +
					"  - `read:activitypub`\n" +
					"  - `write:activitypub`\n" +
					"  - `read:admin`\n" +
					"  - `write:admin`\n" +
					"  - `read:issue`\n" +
					"  - `write:issue`\n" +
					"  - `read:misc`\n" +
					"  - `write:misc`\n" +
					"  - `read:notification`\n" +
					"  - `write:notification`\n" +
					"  - `read:organization`\n" +
					"  - `write:organization`\n" +
					"  - `read:package`\n" +
					"  - `write:package`\n" +
					"  - `read:repository`\n" +
					"  - `write:repository`\n" +
					"  - `read:user`\n" +
					"  - `write:user`",
				ElementType: types.StringType,
				Required:    true,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.RequiresReplace(),
				},
			},
			"repositories": schema.SetAttribute{
				Description: "Set of repositories (`owner/name`) the token is limited to. " +
					"When unset, the token has access to every repository the user can access. " +
					"Requires Forgejo 15 or newer.",
				ElementType: types.StringType,
				Optional:    true,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(
						stringvalidator.RegexMatches(repoFullNameRegexp, "must be of the form owner/name"),
					),
				},
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.RequiresReplace(),
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

	client, ok := req.ProviderData.(*providerData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf(
				"Expected *providerData, got: %T. Please report this issue to the provider developers.",
				req.ProviderData,
			),
		)

		return
	}

	r.client = client.Client
	r.api = client.api
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

	var scopes []string
	diags = data.Scopes.ElementsAs(ctx, &scopes, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var repoNames []string
	if !data.Repositories.IsNull() {
		diags = data.Repositories.ElementsAs(ctx, &repoNames, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	opt := createAccessTokenOption{
		Name:   data.Name.ValueString(),
		Scopes: scopes,
	}
	for _, full := range repoNames {
		owner, name, _ := strings.Cut(full, "/")
		opt.Repositories = append(opt.Repositories, repoTarget{Owner: owner, Name: name})
	}

	// The SDK's CreateAccessToken does not know `repositories`, so call the
	// API directly. Username is required so we always have it.
	username := data.Username.ValueString()
	var token accessToken
	err := r.api.do(ctx, "POST", fmt.Sprintf("/users/%s/tokens", url.PathEscape(username)), opt, &token)
	if err != nil {
		var msg string
		var apiErr *apiError
		if !errors.As(err, &apiErr) {
			msg = fmt.Sprintf("Unknown error with nil response: %s", err)
		} else {
			tflog.Error(ctx, "Error", map[string]any{
				"status": apiErr.Status,
			})

			switch apiErr.StatusCode {
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
		}
		resp.Diagnostics.AddError("Unable to create access token", msg)

		return
	}

	// Map response to model
	data.from(&token)

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

	tflog.Info(ctx, "Read access token", map[string]any{
		"id": tokenID,
	})

	// Username is required, so we always know which user's tokens to list.
	// Listed directly instead of via the SDK to get `repositories` back.
	username := data.Username.ValueString()
	tokens, err := r.listAccessTokens(ctx, username)
	if err != nil {
		var msg string
		var apiErr *apiError
		if !errors.As(err, &apiErr) {
			msg = fmt.Sprintf("Unknown error with nil response: %s", err)
		} else {
			tflog.Error(ctx, "Error", map[string]any{
				"status": apiErr.Status,
			})

			switch apiErr.StatusCode {
			case 401:
				msg = fmt.Sprintf("Unauthorized: authentication required to list access tokens: %s", err)
			case 403:
				msg = fmt.Sprintf("Forbidden: insufficient permissions to list access tokens: %s", err)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to list access tokens", msg)

		return
	}

	// Find token by ID
	var foundToken *accessToken
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

	// Use Forgejo client to delete access token
	res, err := r.client.DeleteAccessToken(username, tokenID)
	if err != nil {
		var msg string
		if res == nil {
			msg = fmt.Sprintf("Unknown error with nil response: %s", err)
		} else {
			tflog.Error(ctx, "Error", map[string]any{
				"status": res.Status,
			})

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
		}
		resp.Diagnostics.AddError("Unable to delete access token", msg)

		return
	}
}

// listAccessTokens returns all access tokens of a user, following pagination.
func (r *accessTokenResource) listAccessTokens(ctx context.Context, username string) ([]*accessToken, error) {
	const pageSize = 50
	var all []*accessToken
	for page := 1; ; page++ {
		var tokens []*accessToken
		path := fmt.Sprintf("/users/%s/tokens?page=%d&limit=%d", url.PathEscape(username), page, pageSize)
		if err := r.api.do(ctx, "GET", path, nil, &tokens); err != nil {
			return nil, err
		}
		all = append(all, tokens...)
		if len(tokens) < pageSize {
			return all, nil
		}
	}
}

// from converts an API access token to the resource model.
func (m *accessTokenResourceModel) from(t *accessToken) {
	m.ID = types.Int64Value(t.ID)
	m.Name = types.StringValue(t.Name)

	// Convert scopes from API
	// Note: Scopes may be reordered by the API, but we preserve them as-is
	if len(t.Scopes) > 0 {
		scopeValues := make([]attr.Value, len(t.Scopes))
		for i, scope := range t.Scopes {
			scopeValues[i] = types.StringValue(scope)
		}
		m.Scopes = types.SetValueMust(types.StringType, scopeValues)
	} else {
		// No scopes returned
		m.Scopes = types.SetNull(types.StringType)
	}

	// Repositories is null when the token is not limited to specific repositories
	if len(t.Repositories) > 0 {
		repoValues := make([]attr.Value, len(t.Repositories))
		for i, repo := range t.Repositories {
			repoValues[i] = types.StringValue(repo.FullName)
		}
		m.Repositories = types.SetValueMust(types.StringType, repoValues)
	} else {
		m.Repositories = types.SetNull(types.StringType)
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
