package provider

import (
	"context"
	"fmt"
	"regexp"
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

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &repositoryActionVariableResource{}
	_ resource.ResourceWithConfigure   = &repositoryActionVariableResource{}
	_ resource.ResourceWithImportState = &repositoryActionVariableResource{}
)

// repositoryActionVariableResource is the resource implementation.
type repositoryActionVariableResource struct {
	client *forgejo.Client
}

// repositoryActionVariableResourceModel maps the resource schema data.
type repositoryActionVariableResourceModel struct {
	RepositoryID types.Int64  `tfsdk:"repository_id"`
	Name         types.String `tfsdk:"name"`
	Value        types.String `tfsdk:"value"`
}

// Metadata returns the resource type name.
func (r *repositoryActionVariableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_repository_action_variable"
}

// Schema defines the schema for the resource.
func (r *repositoryActionVariableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Forgejo repository action variable resource.",

		Attributes: map[string]schema.Attribute{
			"repository_id": schema.Int64Attribute{
				Description: "Numeric identifier of the repository.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the variable. Must contain only uppercase letters, digits, and underscores, and start with a letter.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.RegexMatches(regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`), "must contain only uppercase letters, digits, and underscores, and start with a letter"),
				},
			},
			"value": schema.StringAttribute{
				Description: "Value of the variable.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *repositoryActionVariableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *repositoryActionVariableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer un(trace(ctx, "Create repository action variable resource"))

	var (
		repo repositoryResourceModel
		data repositoryActionVariableResourceModel
	)

	// Read Terraform plan data into model
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Read repository", map[string]any{
		"id": data.RepositoryID.ValueInt64(),
	})

	// Use Forgejo client to get repository by id
	rep, res, err := r.client.GetRepoByID(data.RepositoryID.ValueInt64())
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
					"Repository with id %d not found: %s",
					data.RepositoryID.ValueInt64(),
					err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to read repository", msg)

		return
	}

	// Map response body to model
	repo.from(rep)

	tflog.Info(ctx, "Create repository action variable", map[string]any{
		"user":  repo.Owner.ValueString(),
		"repo":  repo.Name.ValueString(),
		"name":  data.Name.ValueString(),
		"value": data.Value.ValueString(),
	})

	// Generate API request body from plan
	opts := forgejo.CreateVariableOption{
		Name: data.Name.ValueString(),
		Data: data.Value.ValueString(),
	}

	// Validate API request body
	err = opts.Validate()
	if err != nil {
		resp.Diagnostics.AddError("Input validation error", err.Error())

		return
	}

	// Use Forgejo client to create new repository action variable
	res, err = r.client.CreateRepoActionVariable(
		repo.Owner.ValueString(),
		repo.Name.ValueString(),
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
			case 400:
				msg = fmt.Sprintf("Generic error: %s", err)
			case 404:
				msg = fmt.Sprintf(
					"Repository with owner %s and name %s not found: %s",
					repo.Owner.String(),
					repo.Name.String(),
					err,
				)
			case 409:
				msg = fmt.Sprintf(
					"Variable with name %s already exists: %s",
					data.Name.String(),
					err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to create repository action variable", msg)

		return
	}

	// Save data into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *repositoryActionVariableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer un(trace(ctx, "Read repository action variable resource"))

	var (
		repo repositoryResourceModel
		data repositoryActionVariableResourceModel
	)

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Read repository", map[string]any{
		"id": data.RepositoryID.ValueInt64(),
	})

	// Use Forgejo client to get repository by id
	rep, res, err := r.client.GetRepoByID(data.RepositoryID.ValueInt64())
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
					"Repository with id %d not found: %s",
					data.RepositoryID.ValueInt64(),
					err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to read repository", msg)

		return
	}

	// Map response body to model
	repo.from(rep)

	tflog.Info(ctx, "Read repository action variable", map[string]any{
		"user": repo.Owner.ValueString(),
		"repo": repo.Name.ValueString(),
		"name": data.Name.ValueString(),
	})

	// Use Forgejo client to get repository action variable
	variable, res, err := r.client.GetRepoActionVariable(
		repo.Owner.ValueString(),
		repo.Name.ValueString(),
		data.Name.ValueString(),
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
					"Repository action variable with user '%s' repo '%s' and name %s not found: %s",
					repo.Owner.ValueString(),
					repo.Name.ValueString(),
					data.Name.String(),
					err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to read repository action variable", msg)

		return
	}

	data.Name = types.StringValue(variable.Name)
	data.Value = types.StringValue(variable.Data)

	// Save data into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *repositoryActionVariableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer un(trace(ctx, "Update repository action variable resource"))

	var (
		repo repositoryResourceModel
		data repositoryActionVariableResourceModel
	)

	// Read Terraform plan data into model
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Read repository", map[string]any{
		"id": data.RepositoryID.ValueInt64(),
	})

	// Use Forgejo client to get repository by id
	rep, res, err := r.client.GetRepoByID(data.RepositoryID.ValueInt64())
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
					"Repository with id %d not found: %s",
					data.RepositoryID.ValueInt64(),
					err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to read repository", msg)

		return
	}

	// Map response body to model
	repo.from(rep)

	tflog.Info(ctx, "Update repository action variable", map[string]any{
		"user":  repo.Owner.ValueString(),
		"repo":  repo.Name.ValueString(),
		"name":  data.Name.ValueString(),
		"value": data.Value.ValueString(),
	})

	// Generate API request body from plan
	opts := forgejo.CreateVariableOption{
		Name: data.Name.ValueString(),
		Data: data.Value.ValueString(),
	}

	// Validate API request body
	err = opts.Validate()
	if err != nil {
		resp.Diagnostics.AddError("Input validation error", err.Error())

		return
	}

	// Use Forgejo client to update repository action variable
	res, err = r.client.UpdateRepoActionVariable(
		repo.Owner.ValueString(),
		repo.Name.ValueString(),
		data.Name.ValueString(),
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
			case 400:
				msg = fmt.Sprintf("Generic error: %s", err)
			case 404:
				msg = fmt.Sprintf(
					"Repository with owner %s and name %s not found: %s",
					repo.Owner.String(),
					repo.Name.String(),
					err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to update repository action variable", msg)

		return
	}

	// Save data into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *repositoryActionVariableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	defer un(trace(ctx, "Delete repository action variable resource"))

	var (
		repo repositoryResourceModel
		data repositoryActionVariableResourceModel
	)

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Read repository", map[string]any{
		"id": data.RepositoryID.ValueInt64(),
	})

	// Use Forgejo client to get repository by id
	rep, res, err := r.client.GetRepoByID(data.RepositoryID.ValueInt64())
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
					"Repository with id %d not found: %s",
					data.RepositoryID.ValueInt64(),
					err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to read repository", msg)

		return
	}

	// Map response body to model
	repo.from(rep)

	tflog.Info(ctx, "Delete repository action variable", map[string]any{
		"user": repo.Owner.ValueString(),
		"repo": repo.Name.ValueString(),
		"name": data.Name.ValueString(),
	})

	// Use Forgejo client to delete repository action variable
	res, err = r.client.DeleteRepoActionVariable(
		repo.Owner.ValueString(),
		repo.Name.ValueString(),
		data.Name.ValueString(),
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
					"Repository action variable with user '%s' repo '%s' and name %s not found: %s",
					repo.Owner.ValueString(),
					repo.Name.ValueString(),
					data.Name.String(),
					err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to delete repository action variable", msg)

		return
	}
}

// ImportState imports the resource state from Terraform.
func (r *repositoryActionVariableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	defer un(trace(ctx, "Import repository action variable resource"))

	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected format: repository_id:variable_name, got: %s", req.ID),
		)
		return
	}

	var repoID int64
	if _, err := fmt.Sscanf(parts[0], "%d", &repoID); err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected numeric repository_id, got: %s", parts[0]),
		)
		return
	}

	name := parts[1]

	tflog.Info(ctx, "Importing repository action variable", map[string]any{
		"repository_id": repoID,
		"name":          name,
	})

	// Use Forgejo client to get repository by id
	rep, res, err := r.client.GetRepoByID(repoID)
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
					"Repository with id %d not found: %s",
					repoID, err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to import repository action variable", msg)

		return
	}

	// Use Forgejo client to get variable
	variable, res, err := r.client.GetRepoActionVariable(
		rep.Owner.UserName,
		rep.Name,
		name,
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
					"Repository action variable with id %d and name %s not found: %s",
					repoID, name, err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to import repository action variable", msg)

		return
	}

	var data repositoryActionVariableResourceModel
	data.RepositoryID = types.Int64Value(repoID)
	data.Name = types.StringValue(variable.Name)
	data.Value = types.StringValue(variable.Data)

	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// NewRepositoryActionVariableResource is a helper function to simplify the provider implementation.
func NewRepositoryActionVariableResource() resource.Resource {
	return &repositoryActionVariableResource{}
}
