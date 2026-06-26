package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &repositoryRunnerResource{}
	_ resource.ResourceWithConfigure = &repositoryRunnerResource{}
)

// repositoryRunnerResource is the resource implementation.
type repositoryRunnerResource struct {
	client *forgejo.Client
}

// repositoryRunnerResourceModel maps the resource schema data.
type repositoryRunnerResourceModel struct {
	Repository types.String `tfsdk:"repository"`
	Owner      types.String `tfsdk:"owner"`
	Token      types.String `tfsdk:"token"`
}

// parseRepositoryName extracts owner and repo from repository string.
func (m *repositoryRunnerResourceModel) parseRepositoryName() (string, string, error) {
	if m.Repository.IsNull() || m.Repository.IsUnknown() {
		return "", "", nil
	}

	parts := strings.Split(m.Repository.ValueString(), "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("repository must be in format 'owner/repo', got: %s", m.Repository.ValueString())
	}

	return parts[0], parts[1], nil
}

// Metadata returns the resource type name.
func (r *repositoryRunnerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_repository_runner"
}

// Schema defines the schema for the resource.
func (r *repositoryRunnerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Forgejo repository Actions runner registration token. " +
			"Fetches a registration token at repository scope on create and stores it in state. " +
			"The token is single-use: pass it to a runner daemon to register. " +
			"Tokens are not re-fetched on read or revoked on delete — taint the resource to obtain a fresh token.",

		Attributes: map[string]schema.Attribute{
			"repository": schema.StringAttribute{
				Description: "Repository name in format 'owner/repo'.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"owner": schema.StringAttribute{
				Description: "Owner of the repository.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"token": schema.StringAttribute{
				Description: "Runner registration token. Hand to a runner daemon (e.g. via Kubernetes secret or systemd EnvironmentFile).",
				Computed:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *repositoryRunnerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *repositoryRunnerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer un(trace(ctx, "Create repository runner resource"))

	var data repositoryRunnerResourceModel

	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	owner, repo, err := data.parseRepositoryName()
	if err != nil {
		resp.Diagnostics.AddError("Invalid repository format", err.Error())
		return
	}

	tflog.Info(ctx, "Create repository runner", map[string]any{
		"owner": owner,
		"repo":  repo,
	})

	token, res, err := r.client.GetRepoActionRunnerRegistrationToken(owner, repo)
	if err != nil {
		var msg string
		if res == nil {
			msg = fmt.Sprintf("Unknown error with nil response: %s", err)
		} else {
			tflog.Error(ctx, "Error", map[string]any{
				"status": res.Status,
			})

			switch res.StatusCode {
			case 403:
				msg = fmt.Sprintf(
					"Repository runner token for %s/%s forbidden (token needs write:repository scope): %s",
					owner, repo, err,
				)
			case 404:
				msg = fmt.Sprintf(
					"Repository %s/%s not found: %s",
					owner, repo, err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to create repository runner registration token", msg)

		return
	}

	data.Owner = types.StringValue(owner)
	data.Token = types.StringValue(token.Token)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
// The registration token endpoint always returns a fresh token, so we cannot
// verify the stored token without rotating it; Read is a no-op that preserves
// existing state.
func (r *repositoryRunnerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer un(trace(ctx, "Read repository runner resource"))

	var data repositoryRunnerResourceModel
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Update is a no-op: every configurable attribute is ForceNew and computed
// attributes are populated by Create. This handler exists to satisfy the
// resource.Resource interface.
func (r *repositoryRunnerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer un(trace(ctx, "Update repository runner resource"))

	var data repositoryRunnerResourceModel
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Delete removes the resource from state. The Forgejo API does not expose
// token revocation, so this is a no-op on the server.
func (r *repositoryRunnerResource) Delete(ctx context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	defer un(trace(ctx, "Delete repository runner resource"))

	tflog.Warn(ctx, "Deleting repository runner registration token from Terraform state. "+
		"The Forgejo API does not support revoking tokens; the token may still be valid until it expires upstream.")
}

// NewRepositoryRunnerResource is a helper function to simplify the provider implementation.
func NewRepositoryRunnerResource() resource.Resource {
	return &repositoryRunnerResource{}
}
