package provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &repositoryLabelResource{}
	_ resource.ResourceWithConfigure   = &repositoryLabelResource{}
	_ resource.ResourceWithImportState = &repositoryLabelResource{}
)

// repositoryLabelResource is the resource implementation.
type repositoryLabelResource struct {
	client *forgejo.Client
	api    *apiClient
}

// repositoryLabelResourceModel maps the resource schema data.
type repositoryLabelResourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Repository  types.String `tfsdk:"repository"`
	Owner       types.String `tfsdk:"owner"`
	Name        types.String `tfsdk:"name"`
	Color       types.String `tfsdk:"color"`
	Description types.String `tfsdk:"description"`
	Exclusive   types.Bool   `tfsdk:"exclusive"`
	IsArchived  types.Bool   `tfsdk:"is_archived"`
	URL         types.String `tfsdk:"url"`
}

// repositoryLabel is the Forgejo label API object. The SDK's forgejo.Label
// lacks `exclusive` and `is_archived`, so labels go through the raw api client.
type repositoryLabel struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
	Exclusive   bool   `json:"exclusive"`
	IsArchived  bool   `json:"is_archived"`
	URL         string `json:"url"`
}

// createRepositoryLabelOption is the request body for creating a label.
type createRepositoryLabelOption struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
	Exclusive   bool   `json:"exclusive"`
	IsArchived  bool   `json:"is_archived"`
}

// editRepositoryLabelOption is the request body for editing a label.
type editRepositoryLabelOption struct {
	Name        *string `json:"name,omitempty"`
	Color       *string `json:"color,omitempty"`
	Description *string `json:"description,omitempty"`
	Exclusive   *bool   `json:"exclusive,omitempty"`
	IsArchived  *bool   `json:"is_archived,omitempty"`
}

// from converts the Forgejo label API response to the Terraform model.
// Forgejo stores colors without the leading "#" — the schema validator
// rejects the "#" form so state and config stay aligned.
func (m *repositoryLabelResourceModel) from(l *repositoryLabel, repository, owner string) {
	m.ID = types.Int64Value(l.ID)
	m.Repository = types.StringValue(repository)
	m.Owner = types.StringValue(owner)
	m.Name = types.StringValue(l.Name)
	m.Color = types.StringValue(strings.TrimPrefix(l.Color, "#"))
	m.Description = types.StringValue(l.Description)
	m.Exclusive = types.BoolValue(l.Exclusive)
	m.IsArchived = types.BoolValue(l.IsArchived)
	m.URL = types.StringValue(l.URL)
}

// toCreateOption converts the Terraform model to the label create request.
func (m *repositoryLabelResourceModel) toCreateOption() createRepositoryLabelOption {
	return createRepositoryLabelOption{
		Name:        m.Name.ValueString(),
		Color:       m.Color.ValueString(),
		Description: m.Description.ValueString(),
		Exclusive:   m.Exclusive.ValueBool(),
		IsArchived:  m.IsArchived.ValueBool(),
	}
}

// toEditOption converts the Terraform model to the label edit request.
func (m *repositoryLabelResourceModel) toEditOption() editRepositoryLabelOption {
	return editRepositoryLabelOption{
		Name:        forgejo.OptionalString(m.Name.ValueString()),
		Color:       forgejo.OptionalString(m.Color.ValueString()),
		Description: forgejo.OptionalString(m.Description.ValueString()),
		Exclusive:   forgejo.OptionalBool(m.Exclusive.ValueBool()),
		IsArchived:  forgejo.OptionalBool(m.IsArchived.ValueBool()),
	}
}

// repositoryLabelPath returns the API path of a repository's labels, or of a
// single label when id is non-zero.
func repositoryLabelPath(owner, repo string, id int64) string {
	p := fmt.Sprintf("/repos/%s/%s/labels", url.PathEscape(owner), url.PathEscape(repo))
	if id != 0 {
		p += fmt.Sprintf("/%d", id)
	}

	return p
}

// parseRepositoryName extracts owner and repo from repository string.
func (m *repositoryLabelResourceModel) parseRepositoryName() (string, string, error) {
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
func (r *repositoryLabelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_repository_label"
}

// Schema defines the schema for the resource.
func (r *repositoryLabelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Forgejo repository label resource. Manages issue/PR labels on a repository.",

		Attributes: map[string]schema.Attribute{
			"repository": schema.StringAttribute{
				Description: "Repository name in format 'owner/repo'.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Label name.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 50),
				},
			},
			"color": schema.StringAttribute{
				Description: "6-digit hex color of the label, without a leading `#` (e.g. `d73a4a`). Forgejo stores colors in this canonical form.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^[0-9a-fA-F]{6}$`), "must be a 6-digit hex color without a leading #"),
				},
			},
			"description": schema.StringAttribute{
				Description: "Description of the label.",
				Optional:    true,
				Computed:    true,
			},
			"exclusive": schema.BoolAttribute{
				Description: "Whether the label is exclusive. Name the label `scope/item` to make it mutually exclusive with other `scope/` labels. Defaults to `false`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"is_archived": schema.BoolAttribute{
				Description: "Whether the label is archived. Archived labels can no longer be added to issues or pull requests. Defaults to `false`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},

			"id": schema.Int64Attribute{
				Description: "Numeric identifier of the label.",
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"owner": schema.StringAttribute{
				Description: "Owner of the repository.",
				Computed:    true,
			},
			"url": schema.StringAttribute{
				Description: "API URL of the label.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *repositoryLabelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *repositoryLabelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer un(trace(ctx, "Create repository label resource"))

	var data repositoryLabelResourceModel

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

	tflog.Info(ctx, "Create repository label", map[string]any{
		"owner": owner,
		"repo":  repo,
		"name":  data.Name.ValueString(),
		"color": data.Color.ValueString(),
	})

	var label repositoryLabel
	err = r.api.do(ctx, "POST", repositoryLabelPath(owner, repo, 0), data.toCreateOption(), &label)
	if err != nil {
		var msg string
		var res *apiError
		if !errors.As(err, &res) {
			msg = fmt.Sprintf("Unknown error: %s", err)
		} else {
			tflog.Error(ctx, "Error", map[string]any{
				"status": res.Status,
			})

			switch res.StatusCode {
			case 403:
				msg = fmt.Sprintf(
					"Repository label for %s/%s forbidden: %s",
					owner, repo, err,
				)
			case 404:
				msg = fmt.Sprintf(
					"Repository %s/%s not found: %s",
					owner, repo, err,
				)
			case 422:
				msg = fmt.Sprintf("Input validation error: %s", err)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to create repository label", msg)

		return
	}

	data.from(&label, data.Repository.ValueString(), owner)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *repositoryLabelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer un(trace(ctx, "Read repository label resource"))

	var data repositoryLabelResourceModel

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	owner, repo, err := data.parseRepositoryName()
	if err != nil {
		resp.Diagnostics.AddError("Invalid repository format", err.Error())
		return
	}

	tflog.Info(ctx, "Read repository label", map[string]any{
		"owner": owner,
		"repo":  repo,
		"id":    data.ID.ValueInt64(),
	})

	var label repositoryLabel
	err = r.api.do(ctx, "GET", repositoryLabelPath(owner, repo, data.ID.ValueInt64()), nil, &label)
	if err != nil {
		var msg string
		var res *apiError
		if !errors.As(err, &res) {
			msg = fmt.Sprintf("Unknown error: %s", err)
		} else {
			tflog.Error(ctx, "Error", map[string]any{
				"status": res.Status,
			})

			if res.StatusCode == 404 {
				// Forgejo v15 returns 404 instead of 403 for resources the token
				// can't see; only drop state if the repo is still visible (label
				// genuinely gone), otherwise refuse to drop it (likely forbidden).
				if _, repoRes, _ := r.client.GetRepo(owner, repo); repoRes != nil && repoRes.StatusCode == 200 {
					resp.State.RemoveResource(ctx)
					return
				}
				resp.Diagnostics.AddError(
					"Unable to read repository label",
					fmt.Sprintf(
						"Label for repository %q returned 404, but the repository itself is not visible. "+
							"This may be a permission issue rather than a deletion; refusing to drop state automatically. "+
							"Verify access and re-import, or remove it from state if the repository was deleted.",
						data.Repository.ValueString(),
					),
				)
				return
			}

			msg = fmt.Sprintf("Unknown error: %s", err)
		}

		resp.Diagnostics.AddError("Unable to read repository label", msg)

		return
	}

	data.from(&label, data.Repository.ValueString(), owner)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *repositoryLabelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer un(trace(ctx, "Update repository label resource"))

	var data repositoryLabelResourceModel

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

	tflog.Info(ctx, "Update repository label", map[string]any{
		"owner": owner,
		"repo":  repo,
		"id":    data.ID.ValueInt64(),
		"name":  data.Name.ValueString(),
	})

	var label repositoryLabel
	err = r.api.do(ctx, "PATCH", repositoryLabelPath(owner, repo, data.ID.ValueInt64()), data.toEditOption(), &label)
	if err != nil {
		var msg string
		var res *apiError
		if !errors.As(err, &res) {
			msg = fmt.Sprintf("Unknown error: %s", err)
		} else {
			tflog.Error(ctx, "Error", map[string]any{
				"status": res.Status,
			})

			switch res.StatusCode {
			case 403:
				msg = fmt.Sprintf(
					"Repository label for %s/%s id %d forbidden: %s",
					owner, repo, data.ID.ValueInt64(), err,
				)
			case 404:
				msg = fmt.Sprintf(
					"Repository label for %s/%s id %d not found: %s",
					owner, repo, data.ID.ValueInt64(), err,
				)
			case 422:
				msg = fmt.Sprintf("Input validation error: %s", err)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to update repository label", msg)

		return
	}

	data.from(&label, data.Repository.ValueString(), owner)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *repositoryLabelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	defer un(trace(ctx, "Delete repository label resource"))

	var data repositoryLabelResourceModel

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	owner, repo, err := data.parseRepositoryName()
	if err != nil {
		resp.Diagnostics.AddError("Invalid repository format", err.Error())
		return
	}

	tflog.Info(ctx, "Delete repository label", map[string]any{
		"owner": owner,
		"repo":  repo,
		"id":    data.ID.ValueInt64(),
	})

	err = r.api.do(ctx, "DELETE", repositoryLabelPath(owner, repo, data.ID.ValueInt64()), nil, nil)
	if err != nil {
		var msg string
		var res *apiError
		if !errors.As(err, &res) {
			msg = fmt.Sprintf("Unknown error: %s", err)
		} else {
			tflog.Error(ctx, "Error", map[string]any{
				"status": res.Status,
			})

			switch res.StatusCode {
			case 403:
				msg = fmt.Sprintf(
					"Repository label for %s/%s id %d forbidden: %s",
					owner, repo, data.ID.ValueInt64(), err,
				)
			case 404:
				msg = fmt.Sprintf(
					"Repository label for %s/%s id %d not found: %s",
					owner, repo, data.ID.ValueInt64(), err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to delete repository label", msg)

		return
	}
}

// ImportState is called when importing an existing resource.
// The import ID format is: owner:repo:id
// Example: terraform import forgejo_repository_label.bug my-org:my-repo:42.
func (r *repositoryLabelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	defer un(trace(ctx, "Import repository label resource"))

	parts := strings.SplitN(req.ID, ":", 3)
	if len(parts) != 3 {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected format: owner:repo:id, got: %s", req.ID),
		)
		return
	}

	owner := parts[0]
	repo := parts[1]

	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected numeric id, got: %s", parts[2]),
		)
		return
	}

	tflog.Info(ctx, "Importing repository label", map[string]any{
		"owner": owner,
		"repo":  repo,
		"id":    id,
	})

	var label repositoryLabel
	err = r.api.do(ctx, "GET", repositoryLabelPath(owner, repo, id), nil, &label)
	if err != nil {
		var msg string
		var res *apiError
		if !errors.As(err, &res) {
			msg = fmt.Sprintf("Unknown error: %s", err)
		} else {
			tflog.Error(ctx, "Error", map[string]any{
				"status": res.Status,
			})

			switch res.StatusCode {
			case 404:
				msg = fmt.Sprintf(
					"Repository label for %s/%s id %d not found: %s",
					owner, repo, id, err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to import repository label", msg)

		return
	}

	var data repositoryLabelResourceModel
	data.from(&label, fmt.Sprintf("%s/%s", owner, repo), owner)

	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// NewRepositoryLabelResource is a helper function to simplify the provider implementation.
func NewRepositoryLabelResource() resource.Resource {
	return &repositoryLabelResource{}
}
