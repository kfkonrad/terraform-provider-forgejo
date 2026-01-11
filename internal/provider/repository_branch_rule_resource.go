package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &repositoryBranchRuleResource{}
	_ resource.ResourceWithConfigure   = &repositoryBranchRuleResource{}
	_ resource.ResourceWithImportState = &repositoryBranchRuleResource{}
)

// repositoryBranchRuleResource is the resource implementation.
type repositoryBranchRuleResource struct {
	client *forgejo.Client
}

// repositoryBranchRuleResourceModel maps the resource schema data.
type repositoryBranchRuleResourceModel struct {
	Repository                    types.String `tfsdk:"repository"`
	Owner                         types.String `tfsdk:"owner"`
	ProtectedBranchPattern        types.String `tfsdk:"protected_branch_pattern"`
	BranchName                    types.String `tfsdk:"branch_name"`
	EnablePush                    types.Bool   `tfsdk:"enable_push"`
	EnablePushWhitelist           types.Bool   `tfsdk:"enable_push_whitelist"`
	PushWhitelistUsernames        types.Set    `tfsdk:"push_whitelist_usernames"`
	PushWhitelistTeams            types.Set    `tfsdk:"push_whitelist_teams"`
	PushWhitelistDeployKeys       types.Bool   `tfsdk:"push_whitelist_deploy_keys"`
	RequireSignedCommits          types.Bool   `tfsdk:"require_signed_commits"`
	RequiredApprovals             types.Int64  `tfsdk:"required_approvals"`
	EnableApprovalsWhitelist      types.Bool   `tfsdk:"enable_approvals_whitelist"`
	ApprovalsWhitelistUsernames   types.Set    `tfsdk:"approvals_whitelist_usernames"`
	ApprovalsWhitelistTeams       types.Set    `tfsdk:"approvals_whitelist_teams"`
	DismissStaleApprovals         types.Bool   `tfsdk:"dismiss_stale_approvals"`
	EnableStatusCheck             types.Bool   `tfsdk:"enable_status_check"`
	StatusCheckContexts           types.Set    `tfsdk:"status_check_contexts"`
	EnableMergeWhitelist          types.Bool   `tfsdk:"enable_merge_whitelist"`
	MergeWhitelistUsernames       types.Set    `tfsdk:"merge_whitelist_usernames"`
	MergeWhitelistTeams           types.Set    `tfsdk:"merge_whitelist_teams"`
	BlockOnRejectedReviews        types.Bool   `tfsdk:"block_on_rejected_reviews"`
	BlockOnOfficialReviewRequests types.Bool   `tfsdk:"block_on_official_review_requests"`
	BlockOnOutdatedBranch         types.Bool   `tfsdk:"block_on_outdated_branch"`
	ProtectedFilePatterns         types.String `tfsdk:"protected_file_patterns"`
	UnprotectedFilePatterns       types.String `tfsdk:"unprotected_file_patterns"`
	CreatedAt                     types.String `tfsdk:"created_at"`
	UpdatedAt                     types.String `tfsdk:"updated_at"`
}

// from converts the Forgejo BranchProtection API response to the Terraform model.
func (m *repositoryBranchRuleResourceModel) from(bp *forgejo.BranchProtection, repository, owner string) {
	m.Repository = types.StringValue(repository)
	m.Owner = types.StringValue(owner)
	m.ProtectedBranchPattern = types.StringValue(bp.RuleName)
	m.BranchName = types.StringValue(bp.BranchName)
	m.EnablePush = types.BoolValue(bp.EnablePush)
	m.EnablePushWhitelist = types.BoolValue(bp.EnablePushWhitelist)
	m.PushWhitelistUsernames = stringSliceToSet(bp.PushWhitelistUsernames)
	m.PushWhitelistTeams = stringSliceToSet(bp.PushWhitelistTeams)
	m.PushWhitelistDeployKeys = types.BoolValue(bp.PushWhitelistDeployKeys)
	m.RequireSignedCommits = types.BoolValue(bp.RequireSignedCommits)
	m.RequiredApprovals = types.Int64Value(bp.RequiredApprovals)
	m.EnableApprovalsWhitelist = types.BoolValue(bp.EnableApprovalsWhitelist)
	m.ApprovalsWhitelistUsernames = stringSliceToSet(bp.ApprovalsWhitelistUsernames)
	m.ApprovalsWhitelistTeams = stringSliceToSet(bp.ApprovalsWhitelistTeams)
	m.DismissStaleApprovals = types.BoolValue(bp.DismissStaleApprovals)
	m.EnableStatusCheck = types.BoolValue(bp.EnableStatusCheck)
	m.StatusCheckContexts = stringSliceToSet(bp.StatusCheckContexts)
	m.EnableMergeWhitelist = types.BoolValue(bp.EnableMergeWhitelist)
	m.MergeWhitelistUsernames = stringSliceToSet(bp.MergeWhitelistUsernames)
	m.MergeWhitelistTeams = stringSliceToSet(bp.MergeWhitelistTeams)
	m.BlockOnRejectedReviews = types.BoolValue(bp.BlockOnRejectedReviews)
	m.BlockOnOfficialReviewRequests = types.BoolValue(bp.BlockOnOfficialReviewRequests)
	m.BlockOnOutdatedBranch = types.BoolValue(bp.BlockOnOutdatedBranch)
	m.ProtectedFilePatterns = types.StringValue(bp.ProtectedFilePatterns)
	m.UnprotectedFilePatterns = types.StringValue(bp.UnprotectedFilePatterns)
	m.CreatedAt = types.StringValue(bp.Created.String())
	m.UpdatedAt = types.StringValue(bp.Updated.String())
}

// toCreateOption converts the Terraform model to Forgejo CreateBranchProtectionOption.
func (m *repositoryBranchRuleResourceModel) toCreateOption(ctx context.Context) (forgejo.CreateBranchProtectionOption, error) {
	opt := forgejo.CreateBranchProtectionOption{
		RuleName:                      m.ProtectedBranchPattern.ValueString(),
		EnablePush:                    m.EnablePush.ValueBool(),
		EnablePushWhitelist:           m.EnablePushWhitelist.ValueBool(),
		PushWhitelistDeployKeys:       m.PushWhitelistDeployKeys.ValueBool(),
		RequireSignedCommits:          m.RequireSignedCommits.ValueBool(),
		RequiredApprovals:             m.RequiredApprovals.ValueInt64(),
		EnableApprovalsWhitelist:      m.EnableApprovalsWhitelist.ValueBool(),
		DismissStaleApprovals:         m.DismissStaleApprovals.ValueBool(),
		EnableStatusCheck:             m.EnableStatusCheck.ValueBool(),
		EnableMergeWhitelist:          m.EnableMergeWhitelist.ValueBool(),
		BlockOnRejectedReviews:        m.BlockOnRejectedReviews.ValueBool(),
		BlockOnOfficialReviewRequests: m.BlockOnOfficialReviewRequests.ValueBool(),
		BlockOnOutdatedBranch:         m.BlockOnOutdatedBranch.ValueBool(),
		ProtectedFilePatterns:         m.ProtectedFilePatterns.ValueString(),
		UnprotectedFilePatterns:       m.UnprotectedFilePatterns.ValueString(),
	}

	var err error

	opt.PushWhitelistUsernames, err = setToStringSlice(ctx, m.PushWhitelistUsernames)
	if err != nil {
		return opt, err
	}
	opt.PushWhitelistTeams, err = setToStringSlice(ctx, m.PushWhitelistTeams)
	if err != nil {
		return opt, err
	}
	opt.ApprovalsWhitelistUsernames, err = setToStringSlice(ctx, m.ApprovalsWhitelistUsernames)
	if err != nil {
		return opt, err
	}
	opt.ApprovalsWhitelistTeams, err = setToStringSlice(ctx, m.ApprovalsWhitelistTeams)
	if err != nil {
		return opt, err
	}
	opt.StatusCheckContexts, err = setToStringSlice(ctx, m.StatusCheckContexts)
	if err != nil {
		return opt, err
	}
	opt.MergeWhitelistUsernames, err = setToStringSlice(ctx, m.MergeWhitelistUsernames)
	if err != nil {
		return opt, err
	}
	opt.MergeWhitelistTeams, err = setToStringSlice(ctx, m.MergeWhitelistTeams)
	if err != nil {
		return opt, err
	}

	return opt, nil
}

// toEditOption converts the Terraform model to Forgejo EditBranchProtectionOption.
func (m *repositoryBranchRuleResourceModel) toEditOption(ctx context.Context) (forgejo.EditBranchProtectionOption, error) {
	opt := forgejo.EditBranchProtectionOption{
		EnablePush:                    forgejo.OptionalBool(m.EnablePush.ValueBool()),
		EnablePushWhitelist:           forgejo.OptionalBool(m.EnablePushWhitelist.ValueBool()),
		PushWhitelistDeployKeys:       forgejo.OptionalBool(m.PushWhitelistDeployKeys.ValueBool()),
		RequireSignedCommits:          forgejo.OptionalBool(m.RequireSignedCommits.ValueBool()),
		RequiredApprovals:             forgejo.OptionalInt64(m.RequiredApprovals.ValueInt64()),
		EnableApprovalsWhitelist:      forgejo.OptionalBool(m.EnableApprovalsWhitelist.ValueBool()),
		DismissStaleApprovals:         forgejo.OptionalBool(m.DismissStaleApprovals.ValueBool()),
		EnableStatusCheck:             forgejo.OptionalBool(m.EnableStatusCheck.ValueBool()),
		EnableMergeWhitelist:          forgejo.OptionalBool(m.EnableMergeWhitelist.ValueBool()),
		BlockOnRejectedReviews:        forgejo.OptionalBool(m.BlockOnRejectedReviews.ValueBool()),
		BlockOnOfficialReviewRequests: forgejo.OptionalBool(m.BlockOnOfficialReviewRequests.ValueBool()),
		BlockOnOutdatedBranch:         forgejo.OptionalBool(m.BlockOnOutdatedBranch.ValueBool()),
		ProtectedFilePatterns:         forgejo.OptionalString(m.ProtectedFilePatterns.ValueString()),
		UnprotectedFilePatterns:       forgejo.OptionalString(m.UnprotectedFilePatterns.ValueString()),
	}

	var err error

	opt.PushWhitelistUsernames, err = setToStringSlice(ctx, m.PushWhitelistUsernames)
	if err != nil {
		return opt, err
	}
	opt.PushWhitelistTeams, err = setToStringSlice(ctx, m.PushWhitelistTeams)
	if err != nil {
		return opt, err
	}
	opt.ApprovalsWhitelistUsernames, err = setToStringSlice(ctx, m.ApprovalsWhitelistUsernames)
	if err != nil {
		return opt, err
	}
	opt.ApprovalsWhitelistTeams, err = setToStringSlice(ctx, m.ApprovalsWhitelistTeams)
	if err != nil {
		return opt, err
	}
	opt.StatusCheckContexts, err = setToStringSlice(ctx, m.StatusCheckContexts)
	if err != nil {
		return opt, err
	}
	opt.MergeWhitelistUsernames, err = setToStringSlice(ctx, m.MergeWhitelistUsernames)
	if err != nil {
		return opt, err
	}
	opt.MergeWhitelistTeams, err = setToStringSlice(ctx, m.MergeWhitelistTeams)
	if err != nil {
		return opt, err
	}

	return opt, nil
}

// parseRepositoryName extracts owner and repo from repository string.
func (m *repositoryBranchRuleResourceModel) parseRepositoryName() (string, string, error) {
	if m.Repository.IsNull() || m.Repository.IsUnknown() {
		return "", "", nil
	}

	parts := strings.Split(m.Repository.ValueString(), "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("repository must be in format 'owner/repo', got: %s", m.Repository.ValueString())
	}

	return parts[0], parts[1], nil
}

// stringSliceToSet converts a Go []string to a Terraform types.Set.
func stringSliceToSet(s []string) types.Set {
	if len(s) > 0 {
		vals := make([]attr.Value, len(s))
		for i, v := range s {
			vals[i] = types.StringValue(v)
		}
		return types.SetValueMust(types.StringType, vals)
	}
	return types.SetNull(types.StringType)
}

// setToStringSlice converts a Terraform types.Set to a Go []string.
func setToStringSlice(ctx context.Context, s types.Set) ([]string, error) {
	if s.IsNull() || s.IsUnknown() {
		return nil, nil
	}
	var result []string
	diags := s.ElementsAs(ctx, &result, false)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to convert set to string slice: %s", diags.Errors()[0].Detail())
	}
	return result, nil
}

// Metadata returns the resource type name.
func (r *repositoryBranchRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_repository_branch_rule"
}

// Schema defines the schema for the resource.
func (r *repositoryBranchRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Forgejo repository branch rule resource. Manages branch protection rules for repositories.",

		Attributes: map[string]schema.Attribute{
			// Identification (Required, ForceNew)
			"repository": schema.StringAttribute{
				Description: "Repository name in format 'owner/repo'.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"protected_branch_pattern": schema.StringAttribute{
				Description: "Protected branch name pattern (e.g. `main`, `release/**`).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			// Computed (read-only)
			"owner": schema.StringAttribute{
				Description: "Owner of the repository.",
				Computed:    true,
			},
			"branch_name": schema.StringAttribute{
				Description: "The branch name matched by this rule.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "Timestamp of when the branch rule was created.",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "Timestamp of when the branch rule was last updated.",
				Computed:    true,
			},

			// Push section
			"enable_push": schema.BoolAttribute{
				Description: "Enable push to the protected branch. Defaults to `false`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"enable_push_whitelist": schema.BoolAttribute{
				Description: "Whitelist restricted push. Defaults to `false`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"push_whitelist_usernames": schema.SetAttribute{
				Description: "Whitelisted users for pushing.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
			},
			"push_whitelist_teams": schema.SetAttribute{
				Description: "Whitelisted teams for pushing.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
			},
			"push_whitelist_deploy_keys": schema.BoolAttribute{
				Description: "Whitelist deploy keys with write access to push. Defaults to `false`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"require_signed_commits": schema.BoolAttribute{
				Description: "Require signed commits. Defaults to `false`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},

			// Pull request approvals section
			"required_approvals": schema.Int64Attribute{
				Description: "Number of required approvals. Defaults to `0`.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(0),
			},
			"enable_approvals_whitelist": schema.BoolAttribute{
				Description: "Restrict approvals to whitelisted users or teams. Defaults to `false`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"approvals_whitelist_usernames": schema.SetAttribute{
				Description: "Whitelisted reviewers.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
			},
			"approvals_whitelist_teams": schema.SetAttribute{
				Description: "Whitelisted teams for reviews.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
			},
			"dismiss_stale_approvals": schema.BoolAttribute{
				Description: "Dismiss stale approvals when new commits are pushed. Defaults to `false`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},

			// Status checks section
			"enable_status_check": schema.BoolAttribute{
				Description: "Enable status check. Defaults to `false`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"status_check_contexts": schema.SetAttribute{
				Description: "Status check patterns.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
			},

			// Pull request merge section
			"enable_merge_whitelist": schema.BoolAttribute{
				Description: "Enable merge whitelist. Defaults to `false`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"merge_whitelist_usernames": schema.SetAttribute{
				Description: "Whitelisted users for merging.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
			},
			"merge_whitelist_teams": schema.SetAttribute{
				Description: "Whitelisted teams for merging.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
			},
			"block_on_rejected_reviews": schema.BoolAttribute{
				Description: "Block merge on rejected reviews. Defaults to `false`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"block_on_official_review_requests": schema.BoolAttribute{
				Description: "Block merge on official review requests. Defaults to `false`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"block_on_outdated_branch": schema.BoolAttribute{
				Description: "Block merge if pull request is outdated. Defaults to `false`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},

			// File patterns section
			"protected_file_patterns": schema.StringAttribute{
				Description: "Protected file patterns (semicolon-separated glob patterns). Defaults to `\"\"`.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"unprotected_file_patterns": schema.StringAttribute{
				Description: "Unprotected file patterns (semicolon-separated glob patterns). Defaults to `\"\"`.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *repositoryBranchRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *repositoryBranchRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer un(trace(ctx, "Create repository branch rule resource"))

	var data repositoryBranchRuleResourceModel

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

	tflog.Info(ctx, "Create repository branch rule", map[string]any{
		"owner":   owner,
		"repo":    repo,
		"pattern": data.ProtectedBranchPattern.ValueString(),
	})

	opts, err := data.toCreateOption(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to build create options", err.Error())
		return
	}

	bp, res, err := r.client.CreateBranchProtection(owner, repo, opts)
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
					"Repository branch rule for %s/%s forbidden: %s",
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
		resp.Diagnostics.AddError("Unable to create repository branch rule", msg)

		return
	}

	data.from(bp, data.Repository.ValueString(), owner)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *repositoryBranchRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer un(trace(ctx, "Read repository branch rule resource"))

	var data repositoryBranchRuleResourceModel

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

	tflog.Info(ctx, "Read repository branch rule", map[string]any{
		"owner":   owner,
		"repo":    repo,
		"pattern": data.ProtectedBranchPattern.ValueString(),
	})

	bp, res, err := r.client.GetBranchProtection(owner, repo, data.ProtectedBranchPattern.ValueString())
	if err != nil {
		var msg string
		if res == nil {
			msg = fmt.Sprintf("Unknown error with nil response: %s", err)
		} else {
			tflog.Error(ctx, "Error", map[string]any{
				"status": res.Status,
			})

			if res.StatusCode == 404 {
				resp.State.RemoveResource(ctx)
				return
			}

			msg = fmt.Sprintf("Unknown error: %s", err)
		}

		resp.Diagnostics.AddError(
			"Unable to read repository branch rule",
			msg,
		)

		return
	}

	data.from(bp, data.Repository.ValueString(), owner)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *repositoryBranchRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer un(trace(ctx, "Update repository branch rule resource"))

	var data repositoryBranchRuleResourceModel

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

	tflog.Info(ctx, "Update repository branch rule", map[string]any{
		"owner":   owner,
		"repo":    repo,
		"pattern": data.ProtectedBranchPattern.ValueString(),
	})

	opts, err := data.toEditOption(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to build edit options", err.Error())
		return
	}

	bp, res, err := r.client.EditBranchProtection(owner, repo, data.ProtectedBranchPattern.ValueString(), opts)
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
					"Repository branch rule for %s/%s pattern %s forbidden: %s",
					owner, repo, data.ProtectedBranchPattern.ValueString(), err,
				)
			case 404:
				msg = fmt.Sprintf(
					"Repository branch rule for %s/%s pattern %s not found: %s",
					owner, repo, data.ProtectedBranchPattern.ValueString(), err,
				)
			case 422:
				msg = fmt.Sprintf("Input validation error: %s", err)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to update repository branch rule", msg)

		return
	}

	data.from(bp, data.Repository.ValueString(), owner)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *repositoryBranchRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	defer un(trace(ctx, "Delete repository branch rule resource"))

	var data repositoryBranchRuleResourceModel

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

	tflog.Info(ctx, "Delete repository branch rule", map[string]any{
		"owner":   owner,
		"repo":    repo,
		"pattern": data.ProtectedBranchPattern.ValueString(),
	})

	res, err := r.client.DeleteBranchProtection(owner, repo, data.ProtectedBranchPattern.ValueString())
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
					"Repository branch rule for %s/%s pattern %s forbidden: %s",
					owner, repo, data.ProtectedBranchPattern.ValueString(), err,
				)
			case 404:
				msg = fmt.Sprintf(
					"Repository branch rule for %s/%s pattern %s not found: %s",
					owner, repo, data.ProtectedBranchPattern.ValueString(), err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to delete repository branch rule", msg)

		return
	}
}

// ImportState is called when importing an existing resource.
// The import ID format is: owner:repo:protected_branch_pattern
// Example: terraform import forgejo_repository_branch_rule.main my-org:my-repo:main.
func (r *repositoryBranchRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	defer un(trace(ctx, "Import repository branch rule resource"))

	parts := strings.SplitN(req.ID, ":", 3)
	if len(parts) != 3 {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected format: owner:repo:protected_branch_pattern, got: %s", req.ID),
		)
		return
	}

	owner := parts[0]
	repo := parts[1]
	pattern := parts[2]

	tflog.Info(ctx, "Importing repository branch rule", map[string]any{
		"owner":   owner,
		"repo":    repo,
		"pattern": pattern,
	})

	bp, res, err := r.client.GetBranchProtection(owner, repo, pattern)
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
					"Repository branch rule for %s/%s pattern %s not found: %s",
					owner, repo, pattern, err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to import repository branch rule", msg)

		return
	}

	var data repositoryBranchRuleResourceModel
	data.from(bp, fmt.Sprintf("%s/%s", owner, repo), owner)

	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// NewRepositoryBranchRuleResource is a helper function to simplify the provider implementation.
func NewRepositoryBranchRuleResource() resource.Resource {
	return &repositoryBranchRuleResource{}
}
