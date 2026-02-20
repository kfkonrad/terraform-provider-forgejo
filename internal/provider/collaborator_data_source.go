package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &collaboratorDataSource{}
	_ datasource.DataSourceWithConfigure = &collaboratorDataSource{}
)

// collaboratorDataSource is the data source implementation.
type collaboratorDataSource struct {
	client *forgejo.Client
}

// collaboratorDataSourceModel maps the data source schema data.
type collaboratorDataSourceModel struct {
	Owner      types.String `tfsdk:"owner"`
	Repository types.String `tfsdk:"repository"`
	User       types.String `tfsdk:"user"`
	Permission types.String `tfsdk:"permission"`
}

// Metadata returns the data source type name.
func (d *collaboratorDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_collaborator"
}

// Schema defines the schema for the data source.
func (d *collaboratorDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Forgejo collaborator data source.",

		Attributes: map[string]schema.Attribute{
			"owner": schema.StringAttribute{
				Description: "Owner of the repository (user or organization name).",
				Required:    true,
			},
			"repository": schema.StringAttribute{
				Description: "Name of the repository.",
				Required:    true,
			},
			"user": schema.StringAttribute{
				Description: "Username of the collaborator.",
				Required:    true,
			},
			"permission": schema.StringAttribute{
				Description: "Repository permissions of the collaborator.",
				Computed:    true,
			},
		},
	}
}

// Configure adds the provider configured client to the data source.
func (d *collaboratorDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*forgejo.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf(
				"Expected *forgejo.Client, got: %T. Please report this issue to the provider developers.",
				req.ProviderData,
			),
		)

		return
	}

	d.client = client
}

// Read refreshes the Terraform state with the latest data.
func (d *collaboratorDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	defer un(trace(ctx, "Read collaborator data source"))

	var data collaboratorDataSourceModel

	// Read Terraform configuration data into model
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	owner := data.Owner.ValueString()
	repoName := data.Repository.ValueString()

	tflog.Info(ctx, "Read collaborator", map[string]any{
		"owner":        owner,
		"repo":         repoName,
		"collaborator": data.User.ValueString(),
	})

	// Use Forgejo client to get collaborator permission
	perms, res, err := d.client.CollaboratorPermission(owner, repoName, data.User.ValueString())
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
					"Collaborator with user %s repo %s/%s forbidden: %s",
					data.User.String(),
					owner,
					repoName,
					err,
				)
			case 404:
				msg = fmt.Sprintf(
					"Collaborator with user %s repo %s/%s not found: %s",
					data.User.String(),
					owner,
					repoName,
					err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to read collaborator", msg)

		return
	}

	// Map response body to model
	data.Permission = types.StringValue(string(perms.Permission))

	// Save data into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// NewCollaboratorDataSource is a helper function to simplify the provider implementation.
func NewCollaboratorDataSource() datasource.DataSource {
	return &collaboratorDataSource{}
}
