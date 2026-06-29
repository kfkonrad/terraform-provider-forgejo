package provider

import (
	"context"
	"fmt"

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
	_ resource.Resource              = &organizationRunnerResource{}
	_ resource.ResourceWithConfigure = &organizationRunnerResource{}
)

// organizationRunnerResource is the resource implementation.
type organizationRunnerResource struct {
	client *forgejo.Client
}

// organizationRunnerResourceModel maps the resource schema data.
type organizationRunnerResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	Token        types.String `tfsdk:"token"`
}

// Metadata returns the resource type name.
func (r *organizationRunnerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_runner"
}

// Schema defines the schema for the resource.
func (r *organizationRunnerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Forgejo organization Actions runner registration token. " +
			"Fetches a registration token at organization scope on create and stores it in state. " +
			"The token is single-use: pass it to a runner daemon to register. " +
			"Tokens are not re-fetched on read or revoked on delete — taint the resource to obtain a fresh token.",

		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				Description: "Name of the organization.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
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
func (r *organizationRunnerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *organizationRunnerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer un(trace(ctx, "Create organization runner resource"))

	var data organizationRunnerResourceModel

	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	org := data.Organization.ValueString()

	tflog.Info(ctx, "Create organization runner", map[string]any{
		"organization": org,
	})

	token, res, err := r.client.GetOrgActionRunnerRegistrationToken(org)
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
					"Organization runner token for %s forbidden (token needs write:organization scope): %s",
					org, err,
				)
			case 404:
				msg = fmt.Sprintf(
					"Organization %s not found: %s",
					org, err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to create organization runner registration token", msg)

		return
	}

	data.Token = types.StringValue(token.Token)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
// The registration token endpoint always returns a fresh token, so we cannot
// verify the stored token without rotating it; Read is a no-op that preserves
// existing state.
func (r *organizationRunnerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer un(trace(ctx, "Read organization runner resource"))

	var data organizationRunnerResourceModel
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
func (r *organizationRunnerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer un(trace(ctx, "Update organization runner resource"))

	var data organizationRunnerResourceModel
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
func (r *organizationRunnerResource) Delete(ctx context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	defer un(trace(ctx, "Delete organization runner resource"))

	tflog.Warn(ctx, "Deleting organization runner registration token from Terraform state. "+
		"The Forgejo API does not support revoking tokens; the token may still be valid until it expires upstream.")
}

// NewOrganizationRunnerResource is a helper function to simplify the provider implementation.
func NewOrganizationRunnerResource() resource.Resource {
	return &organizationRunnerResource{}
}
