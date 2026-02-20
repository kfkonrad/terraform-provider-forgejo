package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &repositoryWebhookResource{}
	_ resource.ResourceWithConfigure   = &repositoryWebhookResource{}
	_ resource.ResourceWithImportState = &repositoryWebhookResource{}
)

// repositoryWebhookResource is the resource implementation.
type repositoryWebhookResource struct {
	client *forgejo.Client
}

// repositoryWebhookResourceModel maps the resource schema data.
// https://pkg.go.dev/codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2#Hook
type repositoryWebhookResourceModel struct {
	ID                  types.Int64  `tfsdk:"id"`
	Repository          types.String `tfsdk:"repository"`
	Owner               types.String `tfsdk:"owner"`
	Type                types.String `tfsdk:"type"`
	URL                 types.String `tfsdk:"url"`
	ContentType         types.String `tfsdk:"content_type"`
	Secret              types.String `tfsdk:"secret"`
	AuthorizationHeader types.String `tfsdk:"authorization_header"`
	Events              types.Object `tfsdk:"events"`
	Active              types.Bool   `tfsdk:"active"`
	BranchFilter        types.String `tfsdk:"branch_filter"`
	Config              types.Map    `tfsdk:"config"`
}

// webhookEventsModel represents the events block matching Forgejo's HookEvents struct.
type webhookEventsModel struct {
	Create                   types.Bool `tfsdk:"create"`
	Delete                   types.Bool `tfsdk:"delete"`
	Fork                     types.Bool `tfsdk:"fork"`
	Push                     types.Bool `tfsdk:"push"`
	Issues                   types.Bool `tfsdk:"issues"`
	IssueAssign              types.Bool `tfsdk:"issue_assign"`
	IssueLabel               types.Bool `tfsdk:"issue_label"`
	IssueMilestone           types.Bool `tfsdk:"issue_milestone"`
	IssueComment             types.Bool `tfsdk:"issue_comment"`
	PullRequest              types.Bool `tfsdk:"pull_request"`
	PullRequestAssign        types.Bool `tfsdk:"pull_request_assign"`
	PullRequestLabel         types.Bool `tfsdk:"pull_request_label"`
	PullRequestMilestone     types.Bool `tfsdk:"pull_request_milestone"`
	PullRequestComment       types.Bool `tfsdk:"pull_request_comment"`
	PullRequestReview        types.Bool `tfsdk:"pull_request_review"`
	PullRequestSync          types.Bool `tfsdk:"pull_request_sync"`
	PullRequestReviewRequest types.Bool `tfsdk:"pull_request_review_request"`
	Wiki                     types.Bool `tfsdk:"wiki"`
	Repository               types.Bool `tfsdk:"repository"`
	Release                  types.Bool `tfsdk:"release"`
	Package                  types.Bool `tfsdk:"package"`
	ActionRunFailure         types.Bool `tfsdk:"action_run_failure"`
	ActionRunRecover         types.Bool `tfsdk:"action_run_recover"`
	ActionRunSuccess         types.Bool `tfsdk:"action_run_success"`
}

// webhookEventsAttrTypes returns the attribute types for the webhookEventsModel.
func webhookEventsAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"create":                      types.BoolType,
		"delete":                      types.BoolType,
		"fork":                        types.BoolType,
		"push":                        types.BoolType,
		"issues":                      types.BoolType,
		"issue_assign":                types.BoolType,
		"issue_label":                 types.BoolType,
		"issue_milestone":             types.BoolType,
		"issue_comment":               types.BoolType,
		"pull_request":                types.BoolType,
		"pull_request_assign":         types.BoolType,
		"pull_request_label":          types.BoolType,
		"pull_request_milestone":      types.BoolType,
		"pull_request_comment":        types.BoolType,
		"pull_request_review":         types.BoolType,
		"pull_request_sync":           types.BoolType,
		"pull_request_review_request": types.BoolType,
		"wiki":                        types.BoolType,
		"repository":                  types.BoolType,
		"release":                     types.BoolType,
		"package":                     types.BoolType,
		"action_run_failure":          types.BoolType,
		"action_run_recover":          types.BoolType,
		"action_run_success":          types.BoolType,
	}
}

// eventsModelToStringSlice converts booleans back to API event strings.
func eventsModelToStringSlice(events *webhookEventsModel) []string {
	var result []string

	if events.Create.ValueBool() {
		result = append(result, "create")
	}
	if events.Delete.ValueBool() {
		result = append(result, "delete")
	}
	if events.Fork.ValueBool() {
		result = append(result, "fork")
	}
	if events.Push.ValueBool() {
		result = append(result, "push")
	}
	// Use "issues_only" to avoid composite expansion of all issue sub-events.
	if events.Issues.ValueBool() {
		result = append(result, "issues_only")
	}
	if events.IssueAssign.ValueBool() {
		result = append(result, "issue_assign")
	}
	if events.IssueLabel.ValueBool() {
		result = append(result, "issue_label")
	}
	if events.IssueMilestone.ValueBool() {
		result = append(result, "issue_milestone")
	}
	if events.IssueComment.ValueBool() {
		result = append(result, "issue_comment")
	}
	// Use "pull_request_only" to avoid composite expansion of all PR sub-events.
	if events.PullRequest.ValueBool() {
		result = append(result, "pull_request_only")
	}
	if events.PullRequestAssign.ValueBool() {
		result = append(result, "pull_request_assign")
	}
	if events.PullRequestLabel.ValueBool() {
		result = append(result, "pull_request_label")
	}
	if events.PullRequestMilestone.ValueBool() {
		result = append(result, "pull_request_milestone")
	}
	if events.PullRequestComment.ValueBool() {
		result = append(result, "pull_request_comment")
	}
	if events.PullRequestReview.ValueBool() {
		result = append(result, "pull_request_review")
	}
	if events.PullRequestSync.ValueBool() {
		result = append(result, "pull_request_sync")
	}
	if events.PullRequestReviewRequest.ValueBool() {
		result = append(result, "pull_request_review_request")
	}
	if events.Wiki.ValueBool() {
		result = append(result, "wiki")
	}
	if events.Repository.ValueBool() {
		result = append(result, "repository")
	}
	if events.Release.ValueBool() {
		result = append(result, "release")
	}
	if events.Package.ValueBool() {
		result = append(result, "package")
	}
	if events.ActionRunFailure.ValueBool() {
		result = append(result, "action_run_failure")
	}
	if events.ActionRunRecover.ValueBool() {
		result = append(result, "action_run_recover")
	}
	if events.ActionRunSuccess.ValueBool() {
		result = append(result, "action_run_success")
	}

	return result
}

// from converts the Forgejo Hook API response to the Terraform model.
// It preserves write-only fields (secret, authorization_header) from plan/state,
// only populates events when the block was specified, and only keeps config keys
// that were already in the plan/state.
func (m *repositoryWebhookResourceModel) from(hook *forgejo.Hook, repository, owner string) {
	m.ID = types.Int64Value(hook.ID)
	m.Repository = types.StringValue(repository)
	m.Owner = types.StringValue(owner)
	m.Type = types.StringValue(hook.Type)
	m.Active = types.BoolValue(hook.Active)

	// Extract values from config map
	if url, ok := hook.Config["url"]; ok {
		m.URL = types.StringValue(url)
	} else {
		m.URL = types.StringNull()
	}

	if contentType, ok := hook.Config["content_type"]; ok {
		m.ContentType = types.StringValue(contentType)
	} else {
		m.ContentType = types.StringValue("json") // Default
	}

	// Secret and authorization_header are write-only: the API does not
	// return their actual values, so preserve them from plan/state.
	// (Do not overwrite m.Secret or m.AuthorizationHeader here.)

	// Only populate events when the block was specified (not null).
	// When the block is omitted from config, m.Events is null and should stay that way.
	if !m.Events.IsNull() {
		m.Events = eventsFromHook(hook.Events)
	}

	// Build config map from API response, but only keep keys that were
	// already present in the plan/state to avoid "new element appeared" errors.
	// When m.Config is null (e.g. import, or config not specified), include all API keys.
	var existingConfigKeys map[string]string
	filterConfig := false
	if !m.Config.IsNull() && !m.Config.IsUnknown() {
		existingConfigKeys = make(map[string]string)
		m.Config.ElementsAs(context.Background(), &existingConfigKeys, false)
		filterConfig = true
	}

	configMap := make(map[string]attr.Value)
	for key, value := range hook.Config {
		switch key {
		case "url", "content_type", "secret", "authorization_header":
			// Skip these as they're exposed as top-level attributes
		default:
			if filterConfig {
				if _, exists := existingConfigKeys[key]; !exists {
					continue
				}
			}
			configMap[key] = types.StringValue(value)
		}
	}

	if len(configMap) > 0 {
		config, diags := types.MapValue(types.StringType, configMap)
		if !diags.HasError() {
			m.Config = config
		} else {
			m.Config = types.MapNull(types.StringType)
		}
	} else if !filterConfig {
		// Only set to null when we weren't filtering (import/no config).
		// When filtering, keep the existing value if all keys were filtered out.
		m.Config = types.MapNull(types.StringType)
	}
}

// eventsFromHook converts a Forgejo events string slice to a Terraform object.
func eventsFromHook(hookEvents []string) types.Object {
	eventsModel := webhookEventsModel{
		Create:                   types.BoolValue(false),
		Delete:                   types.BoolValue(false),
		Fork:                     types.BoolValue(false),
		Push:                     types.BoolValue(false),
		Issues:                   types.BoolValue(false),
		IssueAssign:              types.BoolValue(false),
		IssueLabel:               types.BoolValue(false),
		IssueMilestone:           types.BoolValue(false),
		IssueComment:             types.BoolValue(false),
		PullRequest:              types.BoolValue(false),
		PullRequestAssign:        types.BoolValue(false),
		PullRequestLabel:         types.BoolValue(false),
		PullRequestMilestone:     types.BoolValue(false),
		PullRequestComment:       types.BoolValue(false),
		PullRequestReview:        types.BoolValue(false),
		PullRequestSync:          types.BoolValue(false),
		PullRequestReviewRequest: types.BoolValue(false),
		Wiki:                     types.BoolValue(false),
		Repository:               types.BoolValue(false),
		Release:                  types.BoolValue(false),
		Package:                  types.BoolValue(false),
		ActionRunFailure:         types.BoolValue(false),
		ActionRunRecover:         types.BoolValue(false),
		ActionRunSuccess:         types.BoolValue(false),
	}

	for _, event := range hookEvents {
		switch event {
		case "create":
			eventsModel.Create = types.BoolValue(true)
		case "delete":
			eventsModel.Delete = types.BoolValue(true)
		case "fork":
			eventsModel.Fork = types.BoolValue(true)
		case "push":
			eventsModel.Push = types.BoolValue(true)
		case "issues":
			eventsModel.Issues = types.BoolValue(true)
		case "issue_assign":
			eventsModel.IssueAssign = types.BoolValue(true)
		case "issue_label":
			eventsModel.IssueLabel = types.BoolValue(true)
		case "issue_milestone":
			eventsModel.IssueMilestone = types.BoolValue(true)
		case "issue_comment":
			eventsModel.IssueComment = types.BoolValue(true)
		case "pull_request":
			eventsModel.PullRequest = types.BoolValue(true)
		case "pull_request_assign":
			eventsModel.PullRequestAssign = types.BoolValue(true)
		case "pull_request_label":
			eventsModel.PullRequestLabel = types.BoolValue(true)
		case "pull_request_milestone":
			eventsModel.PullRequestMilestone = types.BoolValue(true)
		case "pull_request_comment":
			eventsModel.PullRequestComment = types.BoolValue(true)
		case "pull_request_review", "pull_request_review_approved", "pull_request_review_rejected":
			eventsModel.PullRequestReview = types.BoolValue(true)
		case "pull_request_review_comment":
			eventsModel.PullRequestComment = types.BoolValue(true)
		case "pull_request_sync":
			eventsModel.PullRequestSync = types.BoolValue(true)
		case "pull_request_review_request":
			eventsModel.PullRequestReviewRequest = types.BoolValue(true)
		case "wiki":
			eventsModel.Wiki = types.BoolValue(true)
		case "repository":
			eventsModel.Repository = types.BoolValue(true)
		case "release":
			eventsModel.Release = types.BoolValue(true)
		case "package":
			eventsModel.Package = types.BoolValue(true)
		case "action_run_failure":
			eventsModel.ActionRunFailure = types.BoolValue(true)
		case "action_run_recover":
			eventsModel.ActionRunRecover = types.BoolValue(true)
		case "action_run_success":
			eventsModel.ActionRunSuccess = types.BoolValue(true)
		}
	}

	eventsObj, diags := types.ObjectValueFrom(context.Background(), webhookEventsAttrTypes(), eventsModel)
	if !diags.HasError() {
		return eventsObj
	}
	return types.ObjectNull(webhookEventsAttrTypes())
}

// toCreateOption converts the Terraform model to Forgejo CreateHookOption.
func (m *repositoryWebhookResourceModel) toCreateOption() (*forgejo.CreateHookOption, diag.Diagnostics) {
	opts := &forgejo.CreateHookOption{
		Type:                forgejo.HookType(m.Type.ValueString()),
		Config:              make(map[string]string),
		Events:              []string{},
		Active:              m.Active.ValueBool(),
		BranchFilter:        m.BranchFilter.ValueString(),
		AuthorizationHeader: m.AuthorizationHeader.ValueString(),
	}

	// Set required URL
	opts.Config["url"] = m.URL.ValueString()

	// Set content type (default to json)
	contentType := "json"
	if !m.ContentType.IsNull() && !m.ContentType.IsUnknown() {
		contentType = m.ContentType.ValueString()
	}
	opts.Config["content_type"] = contentType

	// Set secret if provided
	if !m.Secret.IsNull() && !m.Secret.IsUnknown() {
		opts.Config["secret"] = m.Secret.ValueString()
	}

	// Convert events object to string slice
	if !m.Events.IsNull() && !m.Events.IsUnknown() {
		var eventsModel webhookEventsModel
		diags := m.Events.As(context.Background(), &eventsModel, basetypes.ObjectAsOptions{})
		if diags.HasError() {
			return nil, diags
		}
		opts.Events = eventsModelToStringSlice(&eventsModel)
	} else {
		opts.Events = []string{}
	}

	// Add additional config values
	if !m.Config.IsNull() && !m.Config.IsUnknown() {
		configMap := make(map[string]string)
		diags := m.Config.ElementsAs(context.Background(), &configMap, false)
		if diags.HasError() {
			return nil, diags
		}
		for key, value := range configMap {
			opts.Config[key] = value
		}
	}

	return opts, nil
}

// toEditOption converts the Terraform model to Forgejo EditHookOption.
func (m *repositoryWebhookResourceModel) toEditOption() (*forgejo.EditHookOption, diag.Diagnostics) {
	opts := &forgejo.EditHookOption{
		Config:              make(map[string]string),
		Events:              []string{},
		BranchFilter:        m.BranchFilter.ValueString(),
		AuthorizationHeader: m.AuthorizationHeader.ValueString(),
	}

	// Set active status (pointer type for optional update)
	if !m.Active.IsNull() && !m.Active.IsUnknown() {
		active := m.Active.ValueBool()
		opts.Active = &active
	}

	// Set required URL
	opts.Config["url"] = m.URL.ValueString()

	// Set content type (default to json)
	contentType := "json"
	if !m.ContentType.IsNull() && !m.ContentType.IsUnknown() {
		contentType = m.ContentType.ValueString()
	}
	opts.Config["content_type"] = contentType

	// Set secret if provided
	if !m.Secret.IsNull() && !m.Secret.IsUnknown() {
		opts.Config["secret"] = m.Secret.ValueString()
	}

	// Convert events object to string slice
	if !m.Events.IsNull() && !m.Events.IsUnknown() {
		var eventsModel webhookEventsModel
		diags := m.Events.As(context.Background(), &eventsModel, basetypes.ObjectAsOptions{})
		if diags.HasError() {
			return nil, diags
		}
		opts.Events = eventsModelToStringSlice(&eventsModel)
	} else {
		opts.Events = []string{}
	}

	// Add additional config values
	if !m.Config.IsNull() && !m.Config.IsUnknown() {
		configMap := make(map[string]string)
		diags := m.Config.ElementsAs(context.Background(), &configMap, false)
		if diags.HasError() {
			return nil, diags
		}
		for key, value := range configMap {
			opts.Config[key] = value
		}
	}

	return opts, nil
}

// parseRepositoryName extracts owner and repo from repository string.
func (m *repositoryWebhookResourceModel) parseRepositoryName() (string, string, diag.Diagnostics) {
	var diags diag.Diagnostics

	if m.Repository.IsNull() || m.Repository.IsUnknown() {
		return "", "", diags
	}

	parts := strings.Split(m.Repository.ValueString(), "/")
	if len(parts) != 2 {
		diags.AddError(
			"Invalid repository format",
			fmt.Sprintf("Repository must be in format 'owner/repo', got: %s", m.Repository.ValueString()),
		)
		return "", "", diags
	}

	return parts[0], parts[1], diags
}

// Metadata returns the resource type name.
func (r *repositoryWebhookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_repository_webhook"
}

// Schema defines the schema for the resource.
func (r *repositoryWebhookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Forgejo repository webhook resource.",

		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "Numeric identifier of the webhook.",
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"repository": schema.StringAttribute{
				Description: "Repository name in format 'owner/repo'.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"owner": schema.StringAttribute{
				Description: "Owner of the repository.",
				Computed:    true,
			},
			"type": schema.StringAttribute{
				Description: "Type of webhook to create.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						"dingtalk",
						"discord",
						"forgejo",
						"gitea",
						"gogs",
						"msteams",
						"slack",
						"telegram",
						"feishu",
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"url": schema.StringAttribute{
				Description: "URL where payloads will be delivered.",
				Required:    true,
			},
			"content_type": schema.StringAttribute{
				Description: "The content type to deliver the payload. Defaults to `json`.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("json"),
				Validators: []validator.String{
					stringvalidator.OneOf("json", "form"),
				},
			},
			"secret": schema.StringAttribute{
				Description: "Secret used to validate payload signatures.",
				Optional:    true,
				Sensitive:   true,
			},
			"authorization_header": schema.StringAttribute{
				Description: "Authorization header value for webhook delivery.",
				Optional:    true,
				Sensitive:   true,
			},
			"active": schema.BoolAttribute{
				Description: "Whether the webhook is active. Defaults to `true`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"branch_filter": schema.StringAttribute{
				Description: "Branch filter for webhook events. Only triggers on matching branches.",
				Optional:    true,
			},
			"config": schema.MapAttribute{
				Description: "Additional configuration options specific to webhook type.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"events": schema.SingleNestedBlock{
				Description: "Events that trigger the webhook. Each field corresponds to a Forgejo webhook event type. All fields default to `false`.",
				Attributes: map[string]schema.Attribute{
					"create": schema.BoolAttribute{
						Description: "Trigger on repository/tag creation.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"delete": schema.BoolAttribute{
						Description: "Trigger on branch/tag deletion.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"fork": schema.BoolAttribute{
						Description: "Trigger on repository fork.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"push": schema.BoolAttribute{
						Description: "Trigger on push to repository.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"issues": schema.BoolAttribute{
						Description: "Trigger on issue open/close/reopen/edit.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"issue_assign": schema.BoolAttribute{
						Description: "Trigger on issue assignment/unassignment.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"issue_label": schema.BoolAttribute{
						Description: "Trigger on issue label added/removed.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"issue_milestone": schema.BoolAttribute{
						Description: "Trigger on issue milestone added/removed/modified.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"issue_comment": schema.BoolAttribute{
						Description: "Trigger on issue comment added/removed/modified.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"pull_request": schema.BoolAttribute{
						Description: "Trigger on pull request open/close/reopen/edit.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"pull_request_assign": schema.BoolAttribute{
						Description: "Trigger on pull request assignment/unassignment.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"pull_request_label": schema.BoolAttribute{
						Description: "Trigger on pull request label added/removed.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"pull_request_milestone": schema.BoolAttribute{
						Description: "Trigger on pull request milestone added/removed/modified.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"pull_request_comment": schema.BoolAttribute{
						Description: "Trigger on pull request comment added/removed/modified.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"pull_request_review": schema.BoolAttribute{
						Description: "Trigger on pull request review approved/rejected/review comment added.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"pull_request_sync": schema.BoolAttribute{
						Description: "Trigger on pull request sync (new commits pushed/force-pushed).",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"pull_request_review_request": schema.BoolAttribute{
						Description: "Trigger on pull request review request added/removed.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"wiki": schema.BoolAttribute{
						Description: "Trigger on wiki page added/removed/edited/renamed.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"repository": schema.BoolAttribute{
						Description: "Trigger on repository created/deleted.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"release": schema.BoolAttribute{
						Description: "Trigger on release published/updated/deleted.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"package": schema.BoolAttribute{
						Description: "Trigger on package created/deleted.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"action_run_failure": schema.BoolAttribute{
						Description: "Trigger on action run failure.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"action_run_recover": schema.BoolAttribute{
						Description: "Trigger on action run success after the last action run in the same workflow failed.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"action_run_success": schema.BoolAttribute{
						Description: "Trigger on action run success.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *repositoryWebhookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *repositoryWebhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer un(trace(ctx, "Create repository webhook resource"))

	var data repositoryWebhookResourceModel

	// Read Terraform plan data into model
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse repository name to get owner and repo
	owner, repo, diags := data.parseRepositoryName()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Create repository webhook", map[string]any{
		"owner": owner,
		"repo":  repo,
		"type":  data.Type.ValueString(),
		"url":   data.URL.ValueString(),
	})

	// Generate API request body from plan
	opts, diags := data.toCreateOption()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate API request body
	err := opts.Validate()
	if err != nil {
		resp.Diagnostics.AddError("Input validation error", err.Error())
		return
	}

	// Use Forgejo client to create new webhook
	hook, res, err := r.client.CreateRepoHook(owner, repo, *opts)
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
					"Repository webhook with owner %s repo %s forbidden: %s",
					owner,
					repo,
					err,
				)
			case 404:
				msg = fmt.Sprintf(
					"Repository with owner %s and name %s not found: %s",
					owner,
					repo,
					err,
				)
			case 422:
				msg = fmt.Sprintf("Input validation error: %s", err)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to create repository webhook", msg)

		return
	}

	// Map response body to model
	data.from(hook, data.Repository.ValueString(), owner)

	// Save data into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *repositoryWebhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer un(trace(ctx, "Read repository webhook resource"))

	var data repositoryWebhookResourceModel

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse repository name to get owner and repo
	owner, repo, diags := data.parseRepositoryName()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Read repository webhook", map[string]any{
		"owner":      owner,
		"repo":       repo,
		"webhook_id": data.ID.ValueInt64(),
	})

	// Use Forgejo client to get webhook
	hook, res, err := r.client.GetRepoHook(owner, repo, data.ID.ValueInt64())
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
					"Repository webhook with owner %s repo %s and id %d not found: %s",
					owner,
					repo,
					data.ID.ValueInt64(),
					err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to read repository webhook", msg)

		return
	}

	// Map response body to model
	data.from(hook, data.Repository.ValueString(), owner)

	// Save data into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *repositoryWebhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer un(trace(ctx, "Update repository webhook resource"))

	var data repositoryWebhookResourceModel

	// Read Terraform plan data into model
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse repository name to get owner and repo
	owner, repo, diags := data.parseRepositoryName()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Update repository webhook", map[string]any{
		"owner":      owner,
		"repo":       repo,
		"webhook_id": data.ID.ValueInt64(),
	})

	// Generate API request body from plan
	opts, diags := data.toEditOption()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Use Forgejo client to update webhook
	res, err := r.client.EditRepoHook(owner, repo, data.ID.ValueInt64(), *opts)
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
					"Repository webhook with owner %s repo %s and id %d forbidden: %s",
					owner,
					repo,
					data.ID.ValueInt64(),
					err,
				)
			case 404:
				msg = fmt.Sprintf(
					"Repository webhook with owner %s repo %s and id %d not found: %s",
					owner,
					repo,
					data.ID.ValueInt64(),
					err,
				)
			case 422:
				msg = fmt.Sprintf("Input validation error: %s", err)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to update repository webhook", msg)

		return
	}

	// Refresh webhook data after update
	hook, _, err := r.client.GetRepoHook(owner, repo, data.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read updated webhook", err.Error())
		return
	}

	// Map response body to model
	data.from(hook, data.Repository.ValueString(), owner)

	// Save data into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *repositoryWebhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	defer un(trace(ctx, "Delete repository webhook resource"))

	var data repositoryWebhookResourceModel

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse repository name to get owner and repo
	owner, repo, diags := data.parseRepositoryName()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Delete repository webhook", map[string]any{
		"owner":      owner,
		"repo":       repo,
		"webhook_id": data.ID.ValueInt64(),
	})

	// Use Forgejo client to delete webhook
	res, err := r.client.DeleteRepoHook(owner, repo, data.ID.ValueInt64())
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
					"Repository webhook with owner %s repo %s and id %d forbidden: %s",
					owner,
					repo,
					data.ID.ValueInt64(),
					err,
				)
			case 404:
				msg = fmt.Sprintf(
					"Repository webhook with owner %s repo %s and id %d not found: %s",
					owner,
					repo,
					data.ID.ValueInt64(),
					err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to delete repository webhook", msg)

		return
	}
}

// ImportState is called when importing an existing resource.
// The import ID format is: owner:repo:webhook_id
// Example: terraform import forgejo_repository_webhook.my_webhook my-org:my-repo:123.
func (r *repositoryWebhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	defer un(trace(ctx, "Import repository webhook resource"))

	// Parse the import ID
	parts := strings.Split(req.ID, ":")
	if len(parts) != 3 {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected format: owner:repo:webhook_id, got: %s", req.ID),
		)
		return
	}

	owner := parts[0]
	repo := parts[1]
	webhookIDStr := parts[2]

	// Convert webhook ID to int64
	webhookID, err := parseWebhookID(webhookIDStr)
	if err != nil {
		resp.Diagnostics.AddError("Invalid webhook ID", err.Error())
		return
	}

	tflog.Info(ctx, "Importing repository webhook", map[string]any{
		"owner":      owner,
		"repo":       repo,
		"webhook_id": webhookID,
	})

	// Fetch the webhook from Forgejo API
	hook, res, err := r.client.GetRepoHook(owner, repo, webhookID)
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
					"Repository webhook with owner %s repo %s and id %d not found: %s",
					owner,
					repo,
					webhookID,
					err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to import repository webhook", msg)

		return
	}

	// Initialize state model with fetched data.
	// Pre-set Events to non-null so from() will populate it from the API.
	var data repositoryWebhookResourceModel
	data.Events = eventsFromHook(nil)
	data.from(hook, fmt.Sprintf("%s/%s", owner, repo), owner)

	// Save the imported state
	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// parseWebhookID converts webhook ID string to int64.
func parseWebhookID(id string) (int64, error) {
	var webhookID int64
	_, err := fmt.Sscanf(id, "%d", &webhookID)
	if err != nil {
		return 0, fmt.Errorf("invalid webhook ID format: %s", id)
	}
	return webhookID, nil
}

// NewRepositoryWebhookResource is a helper function to simplify the provider implementation.
func NewRepositoryWebhookResource() resource.Resource {
	return &repositoryWebhookResource{}
}
