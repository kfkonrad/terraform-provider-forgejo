package provider

import (
	"context"
	"fmt"
	"slices"
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
	Events              types.List   `tfsdk:"events"`
	Active              types.Bool   `tfsdk:"active"`
	BranchFilter        types.String `tfsdk:"branch_filter"`
	Config              types.Map    `tfsdk:"config"`
}

// from converts the Forgejo Hook API response to the Terraform model.
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

	if secret, ok := hook.Config["secret"]; ok {
		m.Secret = types.StringValue(secret)
	} else {
		m.Secret = types.StringNull()
	}

	if authHeader, ok := hook.Config["authorization_header"]; ok {
		m.AuthorizationHeader = types.StringValue(authHeader)
	} else {
		m.AuthorizationHeader = types.StringNull()
	}

	// Convert events slice to Terraform list
	if len(hook.Events) > 0 {
		eventValues := make([]attr.Value, len(hook.Events))
		for i, event := range hook.Events {
			eventValues[i] = types.StringValue(event)
		}
		events, diags := types.ListValue(types.StringType, eventValues)
		if !diags.HasError() {
			m.Events = events
		} else {
			m.Events = types.ListNull(types.StringType)
		}
	} else {
		m.Events = types.ListNull(types.StringType)
	}

	// Set config map (excluding the fields we expose as top-level attributes)
	configMap := make(map[string]attr.Value)
	for key, value := range hook.Config {
		switch key {
		case "url", "content_type", "secret", "authorization_header":
			// Skip these as they're exposed as top-level attributes
		default:
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
	} else {
		m.Config = types.MapNull(types.StringType)
	}
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

	// Convert events list to slice
	if !m.Events.IsNull() && !m.Events.IsUnknown() {
		var events []string
		diags := m.Events.ElementsAs(context.Background(), &events, false)
		if diags.HasError() {
			return nil, diags
		}
		opts.Events = events
	} else {
		// Default to common events if none specified
		opts.Events = []string{"push", "pull_request", "issues", "release"}
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

	// Convert events list to slice
	if !m.Events.IsNull() && !m.Events.IsUnknown() {
		var events []string
		diags := m.Events.ElementsAs(context.Background(), &events, false)
		if diags.HasError() {
			return nil, diags
		}
		opts.Events = events
	} else {
		// Default to common events if none specified
		opts.Events = []string{"push", "pull_request", "issues", "release"}
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
			"events": schema.ListAttribute{
				Description: "List of events that trigger the webhook. If empty, defaults to common events.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
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
		tflog.Error(ctx, "Error", map[string]any{
			"status": res.Status,
		})

		var msg string
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

	tflog.Info(ctx, "Get repository webhook by id", map[string]any{
		"owner":      owner,
		"repo":       repo,
		"webhook_id": data.ID.ValueInt64(),
	})

	// Use Forgejo client to get webhook
	hook, res, err := r.client.GetRepoHook(owner, repo, data.ID.ValueInt64())
	if err != nil {
		tflog.Error(ctx, "Error", map[string]any{
			"status": res.Status,
		})

		var msg string
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
		resp.Diagnostics.AddError("Unable to get repository webhook", msg)

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
		tflog.Error(ctx, "Error", map[string]any{
			"status": res.Status,
		})

		var msg string
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
		tflog.Error(ctx, "Error", map[string]any{
			"status": res.Status,
		})

		var msg string
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
		tflog.Error(ctx, "Error", map[string]any{
			"status": res.Status,
		})

		var msg string
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
		resp.Diagnostics.AddError("Unable to import repository webhook", msg)

		return
	}

	// Initialize state model with fetched data
	var data repositoryWebhookResourceModel
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
