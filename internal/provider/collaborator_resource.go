package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &collaboratorResource{}
	_ resource.ResourceWithConfigure   = &collaboratorResource{}
	_ resource.ResourceWithImportState = &collaboratorResource{}
)

// collaboratorResource is the resource implementation.
type collaboratorResource struct {
	client *forgejo.Client
}

// collaboratorResourceModel maps the resource schema data.
type collaboratorResourceModel struct {
	RepositoryID types.Int64  `tfsdk:"repository_id"`
	User         types.String `tfsdk:"user"`
	Permission   types.String `tfsdk:"permission"`
}

// Metadata returns the resource type name.
func (r *collaboratorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_collaborator"
}

// Schema defines the schema for the resource.
func (r *collaboratorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Forgejo collaborator resource.",

		Attributes: map[string]schema.Attribute{
			"repository_id": schema.Int64Attribute{
				Description: "Numeric identifier of the repository.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"user": schema.StringAttribute{
				Description: "Username of the collaborator.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"permission": schema.StringAttribute{
				Description: "Repository permissions of the collaborator. Allowed values: `read`, `write`, `admin`.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						"read",
						"write",
						"admin",
					),
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *collaboratorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState implements resource.ResourceWithImportState.
// ImportState is called when importing an existing resource.
// The import ID format is: repository_id:user
// Example: terraform import forgejo_collaborator.example 123:john.
func (r *collaboratorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	defer un(trace(ctx, "Import collaborator resource"))

	// Parse the import ID (format: repository_id:user)
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			fmt.Sprintf("Expected format 'repository_id:user', got: %s", req.ID),
		)
		return
	}

	repositoryIDStr := parts[0]
	username := parts[1]

	// Parse repository ID
	repositoryID, err := strconv.ParseInt(repositoryIDStr, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid repository ID",
			fmt.Sprintf("Repository ID must be a number, got: %s", repositoryIDStr),
		)
		return
	}

	tflog.Info(ctx, "Importing collaborator", map[string]any{
		"repository_id": repositoryID,
		"user":          username,
	})

	// Get repository information
	repo, res, err := r.client.GetRepoByID(repositoryID)
	if err != nil {
		if res != nil {
			tflog.Error(ctx, "Error fetching repository", map[string]any{
				"status": res.Status,
			})
		}

		var msg string
		if res != nil {
			switch res.StatusCode {
			case 404:
				msg = fmt.Sprintf("Repository with ID %d not found", repositoryID)
			default:
				msg = fmt.Sprintf("Error fetching repository: %s", err)
			}
		} else {
			msg = fmt.Sprintf("Error fetching repository: %s", err)
		}
		resp.Diagnostics.AddError("Unable to import collaborator", msg)
		return
	}

	// Get collaborator permission
	perms, res, err := r.client.CollaboratorPermission(repo.Owner.UserName, repo.Name, username)
	if err != nil {
		if res != nil {
			tflog.Error(ctx, "Error fetching collaborator", map[string]any{
				"status": res.Status,
			})
		}

		var msg string
		if res != nil {
			switch res.StatusCode {
			case 404:
				msg = fmt.Sprintf("Collaborator %s not found in repository %s/%s", username, repo.Owner.UserName, repo.Name)
			case 403:
				msg = fmt.Sprintf("Not authorized to access collaborator %s in repository %s/%s", username, repo.Owner.UserName, repo.Name)
			default:
				msg = fmt.Sprintf("Error fetching collaborator: %s", err)
			}
		} else {
			msg = fmt.Sprintf("Error fetching collaborator: %s", err)
		}
		resp.Diagnostics.AddError("Unable to import collaborator", msg)
		return
	}

	// Initialize state model with fetched data
	data := collaboratorResourceModel{
		RepositoryID: types.Int64Value(repositoryID),
		User:         types.StringValue(username),
		Permission:   types.StringValue(string(perms.Permission)),
	}

	// Save the imported state
	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Collaborator imported successfully", map[string]any{
		"repository_id": repositoryID,
		"user":          username,
		"permission":    perms.Permission,
	})
}

// Create creates the resource and sets the initial Terraform state.
func (r *collaboratorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer un(trace(ctx, "Create collaborator resource"))

	var (
		repo repositoryResourceModel
		data collaboratorResourceModel
	)

	// Read Terraform plan data into model
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Get repository by id", map[string]any{
		"id": data.RepositoryID.ValueInt64(),
	})

	// Use Forgejo client to get repository by id
	rep, res, err := r.client.GetRepoByID(data.RepositoryID.ValueInt64())
	if err != nil {
		tflog.Error(ctx, "Error", map[string]any{
			"status": res.Status,
		})

		var msg string
		switch res.StatusCode {
		case 404:
			msg = fmt.Sprintf(
				"Repository with id %d not found: %s",
				data.RepositoryID.ValueInt64(),
				err,
			)
		default:
			msg = fmt.Sprintf("Unknown error: %s", err)
		}
		resp.Diagnostics.AddError("Unable to get repository by id", msg)

		return
	}

	// Map response body to model
	repo.from(rep)

	tflog.Info(ctx, "Create collaborator", map[string]any{
		"owner":        repo.Owner.ValueString(),
		"repo":         repo.Name.ValueString(),
		"collaborator": data.User.ValueString(),
		"permission":   data.Permission.ValueString(),
	})

	// Generate API request body from plan
	am := forgejo.AccessMode(data.Permission.ValueString())
	opts := forgejo.AddCollaboratorOption{Permission: &am}

	// Validate API request body
	err = opts.Validate()
	if err != nil {
		resp.Diagnostics.AddError("Input validation error", err.Error())

		return
	}

	// Use Forgejo client to add new collaborator
	res, err = r.client.AddCollaborator(
		repo.Owner.ValueString(),
		repo.Name.ValueString(),
		data.User.ValueString(),
		opts,
	)
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
					"Collaborator with user %s repo %s and name %s forbidden: %s",
					repo.Owner.String(),
					repo.Name.String(),
					data.User.String(),
					err,
				)
			case 404:
				msg = fmt.Sprintf(
					"Collaborator with user %s repo %s and name %s not found: %s",
					repo.Owner.String(),
					repo.Name.String(),
					data.User.String(),
					err,
				)
			case 422:
				msg = fmt.Sprintf("Input validation error: %s", err)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to create collaborator", msg)

		return
	}

	// Save data into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *collaboratorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer un(trace(ctx, "Read collaborator resource"))

	var (
		repo repositoryResourceModel
		data collaboratorResourceModel
	)

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Get repository by id", map[string]any{
		"id": data.RepositoryID.ValueInt64(),
	})

	// Use Forgejo client to get repository by id
	rep, res, err := r.client.GetRepoByID(data.RepositoryID.ValueInt64())
	if err != nil {
		tflog.Error(ctx, "Error", map[string]any{
			"status": res.Status,
		})

		var msg string
		switch res.StatusCode {
		case 404:
			msg = fmt.Sprintf(
				"Repository with id %d not found: %s",
				data.RepositoryID.ValueInt64(),
				err,
			)
		default:
			msg = fmt.Sprintf("Unknown error: %s", err)
		}
		resp.Diagnostics.AddError("Unable to get repository by id", msg)

		return
	}

	// Map response body to model
	repo.from(rep)

	tflog.Info(ctx, "Read collaborator", map[string]any{
		"owner":        repo.Owner.ValueString(),
		"repo":         repo.Name.ValueString(),
		"collaborator": data.User.ValueString(),
	})

	// Use Forgejo client to get collaborator permission
	perms, res, err := r.client.CollaboratorPermission(
		repo.Owner.ValueString(),
		repo.Name.ValueString(),
		data.User.ValueString(),
	)
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
					"Collaborator with user %s repo %s and name %s forbidden: %s",
					repo.Owner.String(),
					repo.Name.String(),
					data.User.String(),
					err,
				)
			case 404:
				msg = fmt.Sprintf(
					"Collaborator with user %s repo %s and name %s not found: %s",
					repo.Owner.String(),
					repo.Name.String(),
					data.User.String(),
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

// Update updates the resource and sets the updated Terraform state on success.
func (r *collaboratorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer un(trace(ctx, "Update collaborator resource"))

	var (
		repo repositoryResourceModel
		data collaboratorResourceModel
	)

	// Read Terraform plan data into model
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Get repository by id", map[string]any{
		"id": data.RepositoryID.ValueInt64(),
	})

	// Use Forgejo client to get repository by id
	rep, res, err := r.client.GetRepoByID(data.RepositoryID.ValueInt64())
	if err != nil {
		tflog.Error(ctx, "Error", map[string]any{
			"status": res.Status,
		})

		var msg string
		switch res.StatusCode {
		case 404:
			msg = fmt.Sprintf(
				"Repository with id %d not found: %s",
				data.RepositoryID.ValueInt64(),
				err,
			)
		default:
			msg = fmt.Sprintf("Unknown error: %s", err)
		}
		resp.Diagnostics.AddError("Unable to get repository by id", msg)

		return
	}

	// Map response body to model
	repo.from(rep)

	tflog.Info(ctx, "Update collaborator", map[string]any{
		"owner":        repo.Owner.ValueString(),
		"repo":         repo.Name.ValueString(),
		"collaborator": data.User.ValueString(),
		"permission":   data.Permission.ValueString(),
	})

	// Generate API request body from plan
	am := forgejo.AccessMode(data.Permission.ValueString())
	opts := forgejo.AddCollaboratorOption{Permission: &am}

	// Validate API request body
	err = opts.Validate()
	if err != nil {
		resp.Diagnostics.AddError("Input validation error", err.Error())

		return
	}

	// Use Forgejo client to update existing collaborator
	res, err = r.client.AddCollaborator(
		repo.Owner.ValueString(),
		repo.Name.ValueString(),
		data.User.ValueString(),
		opts,
	)
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
					"Collaborator with user %s repo %s and name %s forbidden: %s",
					repo.Owner.String(),
					repo.Name.String(),
					data.User.String(),
					err,
				)
			case 404:
				msg = fmt.Sprintf(
					"Collaborator with user %s repo %s and name %s not found: %s",
					repo.Owner.String(),
					repo.Name.String(),
					data.User.String(),
					err,
				)
			case 422:
				msg = fmt.Sprintf("Input validation error: %s", err)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to update collaborator", msg)

		return
	}

	// Save data into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *collaboratorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	defer un(trace(ctx, "Delete collaborator resource"))

	var (
		repo repositoryResourceModel
		data collaboratorResourceModel
	)

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Get repository by id", map[string]any{
		"id": data.RepositoryID.ValueInt64(),
	})

	// Use Forgejo client to get repository by id
	rep, res, err := r.client.GetRepoByID(data.RepositoryID.ValueInt64())
	if err != nil {
		tflog.Error(ctx, "Error", map[string]any{
			"status": res.Status,
		})

		var msg string
		switch res.StatusCode {
		case 404:
			msg = fmt.Sprintf(
				"Repository with id %d not found: %s",
				data.RepositoryID.ValueInt64(),
				err,
			)
		default:
			msg = fmt.Sprintf("Unknown error: %s", err)
		}
		resp.Diagnostics.AddError("Unable to get repository by id", msg)

		return
	}

	// Map response body to model
	repo.from(rep)

	tflog.Info(ctx, "Delete collaborator", map[string]any{
		"owner":        repo.Owner.ValueString(),
		"repo":         repo.Name.ValueString(),
		"collaborator": data.User.ValueString(),
	})

	// Use Forgejo client to delete existing collaborator
	res, err = r.client.DeleteCollaborator(
		repo.Owner.ValueString(),
		repo.Name.ValueString(),
		data.User.ValueString(),
	)
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
					"Collaborator with user %s repo %s and name %s not found: %s",
					repo.Owner.String(),
					repo.Name.String(),
					data.User.String(),
					err,
				)
			case 422:
				msg = fmt.Sprintf("Input validation error: %s", err)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to delete collaborator", msg)

		return
	}
}

// NewCollaboratorResource is a helper function to simplify the provider implementation.
func NewCollaboratorResource() resource.Resource {
	return &collaboratorResource{}
}
