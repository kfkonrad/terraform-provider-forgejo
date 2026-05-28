package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &repositoryTagRuleResource{}
	_ resource.ResourceWithConfigure   = &repositoryTagRuleResource{}
	_ resource.ResourceWithImportState = &repositoryTagRuleResource{}
)

// repositoryTagRuleResource is the resource implementation.
type repositoryTagRuleResource struct {
	client *forgejo.Client
}

// repositoryTagRuleResourceModel maps the resource schema data.
type repositoryTagRuleResourceModel struct {
	ID                  types.Int64  `tfsdk:"id"`
	Repository          types.String `tfsdk:"repository"`
	Owner               types.String `tfsdk:"owner"`
	ProtectedTagPattern types.String `tfsdk:"protected_tag_pattern"`
	WhitelistUsernames  types.Set    `tfsdk:"whitelist_usernames"`
	WhitelistTeams      types.Set    `tfsdk:"whitelist_teams"`
	CreatedAt           types.String `tfsdk:"created_at"`
	UpdatedAt           types.String `tfsdk:"updated_at"`
}

// from converts the Forgejo TagProtection API response to the Terraform model.
func (m *repositoryTagRuleResourceModel) from(tp *forgejo.TagProtection, repository, owner string) {
	m.ID = types.Int64Value(tp.ID)
	m.Repository = types.StringValue(repository)
	m.Owner = types.StringValue(owner)
	m.ProtectedTagPattern = types.StringValue(tp.NamePattern)
	m.WhitelistUsernames = stringSliceToSet(tp.WhitelistUsernames)
	m.WhitelistTeams = stringSliceToSet(tp.WhitelistTeams)
	m.CreatedAt = types.StringValue(tp.Created.Format(time.RFC3339))
	m.UpdatedAt = types.StringValue(tp.Updated.Format(time.RFC3339))
}

// toCreateOption converts the Terraform model to Forgejo CreateTagProtectionOption.
func (m *repositoryTagRuleResourceModel) toCreateOption(ctx context.Context) (forgejo.CreateTagProtectionOption, error) {
	opt := forgejo.CreateTagProtectionOption{
		NamePattern: m.ProtectedTagPattern.ValueString(),
	}

	var err error

	opt.WhitelistUsernames, err = setToStringSlice(ctx, m.WhitelistUsernames)
	if err != nil {
		return opt, err
	}
	opt.WhitelistTeams, err = setToStringSlice(ctx, m.WhitelistTeams)
	if err != nil {
		return opt, err
	}

	return opt, nil
}

// toEditOption converts the Terraform model to Forgejo EditTagProtectionOption.
func (m *repositoryTagRuleResourceModel) toEditOption(ctx context.Context) (forgejo.EditTagProtectionOption, error) {
	opt := forgejo.EditTagProtectionOption{
		NamePattern: forgejo.OptionalString(m.ProtectedTagPattern.ValueString()),
	}

	var err error

	opt.WhitelistUsernames, err = setToStringSlice(ctx, m.WhitelistUsernames)
	if err != nil {
		return opt, err
	}
	opt.WhitelistTeams, err = setToStringSlice(ctx, m.WhitelistTeams)
	if err != nil {
		return opt, err
	}

	return opt, nil
}

// parseRepositoryName extracts owner and repo from repository string.
func (m *repositoryTagRuleResourceModel) parseRepositoryName() (string, string, error) {
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
func (r *repositoryTagRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_repository_tag_rule"
}

// Schema defines the schema for the resource.
func (r *repositoryTagRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Forgejo repository tag rule resource. Manages tag protection rules for repositories.",

		Attributes: map[string]schema.Attribute{
			// Identification (Required, ForceNew)
			"repository": schema.StringAttribute{
				Description: "Repository name in format 'owner/repo'.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"protected_tag_pattern": schema.StringAttribute{
				Description: "Protected tag name pattern (e.g. `v*`, `release/**`).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			// Computed (read-only)
			"id": schema.Int64Attribute{
				Description: "Numeric identifier of the tag protection rule.",
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"owner": schema.StringAttribute{
				Description: "Owner of the repository.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "Timestamp of when the tag rule was created.",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "Timestamp of when the tag rule was last updated.",
				Computed:    true,
			},

			// Optional
			"whitelist_usernames": schema.SetAttribute{
				Description: "Whitelisted users for creating matching tags.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
			},
			"whitelist_teams": schema.SetAttribute{
				Description: "Whitelisted teams for creating matching tags.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *repositoryTagRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *repositoryTagRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer un(trace(ctx, "Create repository tag rule resource"))

	var data repositoryTagRuleResourceModel

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

	tflog.Info(ctx, "Create repository tag rule", map[string]any{
		"owner":   owner,
		"repo":    repo,
		"pattern": data.ProtectedTagPattern.ValueString(),
	})

	opts, err := data.toCreateOption(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to build create options", err.Error())
		return
	}

	tp, res, err := r.client.CreateTagProtection(owner, repo, opts)
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
					"Repository tag rule for %s/%s forbidden: %s",
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
		resp.Diagnostics.AddError("Unable to create repository tag rule", msg)

		return
	}

	data.from(tp, data.Repository.ValueString(), owner)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *repositoryTagRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer un(trace(ctx, "Read repository tag rule resource"))

	var data repositoryTagRuleResourceModel

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

	tflog.Info(ctx, "Read repository tag rule", map[string]any{
		"owner":   owner,
		"repo":    repo,
		"id":      data.ID.ValueInt64(),
		"pattern": data.ProtectedTagPattern.ValueString(),
	})

	tp, res, err := r.client.GetTagProtection(owner, repo, data.ID.ValueInt64())
	if err != nil {
		var msg string
		if res == nil {
			msg = fmt.Sprintf("Unknown error with nil response: %s", err)
		} else {
			tflog.Error(ctx, "Error", map[string]any{
				"status": res.Status,
			})

			if res.StatusCode == 404 {
				// Forgejo v15 returns 404 instead of 403 for resources the token
				// can't see; only drop state if the repo is still visible (rule
				// genuinely gone), otherwise refuse to drop it (likely forbidden).
				if _, repoRes, _ := r.client.GetRepo(owner, repo); repoRes != nil && repoRes.StatusCode == 200 {
					resp.State.RemoveResource(ctx)
					return
				}
				resp.Diagnostics.AddError(
					"Unable to read repository tag rule",
					fmt.Sprintf(
						"Tag protection for repository %q returned 404, but the repository itself is not visible. "+
							"This may be a permission issue rather than a deletion; refusing to drop state automatically. "+
							"Verify access and re-import, or remove it from state if the repository was deleted.",
						data.Repository.ValueString(),
					),
				)
				return
			}

			msg = fmt.Sprintf("Unknown error: %s", err)
		}

		resp.Diagnostics.AddError(
			"Unable to read repository tag rule",
			msg,
		)

		return
	}

	data.from(tp, data.Repository.ValueString(), owner)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *repositoryTagRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer un(trace(ctx, "Update repository tag rule resource"))

	var data repositoryTagRuleResourceModel

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

	tflog.Info(ctx, "Update repository tag rule", map[string]any{
		"owner":   owner,
		"repo":    repo,
		"id":      data.ID.ValueInt64(),
		"pattern": data.ProtectedTagPattern.ValueString(),
	})

	opts, err := data.toEditOption(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to build edit options", err.Error())
		return
	}

	tp, res, err := r.client.EditTagProtection(owner, repo, data.ID.ValueInt64(), opts)
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
					"Repository tag rule for %s/%s id %d forbidden: %s",
					owner, repo, data.ID.ValueInt64(), err,
				)
			case 404:
				msg = fmt.Sprintf(
					"Repository tag rule for %s/%s id %d not found: %s",
					owner, repo, data.ID.ValueInt64(), err,
				)
			case 422:
				msg = fmt.Sprintf("Input validation error: %s", err)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to update repository tag rule", msg)

		return
	}

	data.from(tp, data.Repository.ValueString(), owner)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *repositoryTagRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	defer un(trace(ctx, "Delete repository tag rule resource"))

	var data repositoryTagRuleResourceModel

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

	tflog.Info(ctx, "Delete repository tag rule", map[string]any{
		"owner":   owner,
		"repo":    repo,
		"id":      data.ID.ValueInt64(),
		"pattern": data.ProtectedTagPattern.ValueString(),
	})

	res, err := r.client.DeleteTagProtection(owner, repo, data.ID.ValueInt64())
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
					"Repository tag rule for %s/%s id %d forbidden: %s",
					owner, repo, data.ID.ValueInt64(), err,
				)
			case 404:
				msg = fmt.Sprintf(
					"Repository tag rule for %s/%s id %d not found: %s",
					owner, repo, data.ID.ValueInt64(), err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to delete repository tag rule", msg)

		return
	}
}

// ImportState is called when importing an existing resource.
// The import ID format is: owner:repo:id
// Example: terraform import forgejo_repository_tag_rule.release my-org:my-repo:42.
func (r *repositoryTagRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	defer un(trace(ctx, "Import repository tag rule resource"))

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

	var id int64
	if _, err := fmt.Sscanf(parts[2], "%d", &id); err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected numeric id, got: %s", parts[2]),
		)
		return
	}

	tflog.Info(ctx, "Importing repository tag rule", map[string]any{
		"owner": owner,
		"repo":  repo,
		"id":    id,
	})

	tp, res, err := r.client.GetTagProtection(owner, repo, id)
	if err != nil {
		var msg string
		if res == nil {
			msg = fmt.Sprintf("Unknown error with nil response: %s", err)
		} else {
			tflog.Error(ctx, "Error", map[string]any{
				"status": res.Status,
			})

			switch res.StatusCode {
			case 404:
				msg = fmt.Sprintf(
					"Repository tag rule for %s/%s id %d not found: %s",
					owner, repo, id, err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to import repository tag rule", msg)

		return
	}

	var data repositoryTagRuleResourceModel
	data.from(tp, fmt.Sprintf("%s/%s", owner, repo), owner)

	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// NewRepositoryTagRuleResource is a helper function to simplify the provider implementation.
func NewRepositoryTagRuleResource() resource.Resource {
	return &repositoryTagRuleResource{}
}
