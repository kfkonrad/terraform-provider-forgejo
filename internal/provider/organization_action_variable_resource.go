package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &organizationActionVariableResource{}
	_ resource.ResourceWithConfigure   = &organizationActionVariableResource{}
	_ resource.ResourceWithImportState = &organizationActionVariableResource{}
)

// organizationActionVariableResource is the resource implementation.
type organizationActionVariableResource struct {
	client *forgejo.Client
}

// organizationActionVariableResourceModel maps the resource schema data.
type organizationActionVariableResourceModel struct {
	Organization types.String `tfsdk:"organization"`
	Name         types.String `tfsdk:"name"`
	Value        types.String `tfsdk:"value"`
}

// Metadata returns the resource type name.
func (r *organizationActionVariableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_action_variable"
}

// Schema defines the schema for the resource.
func (r *organizationActionVariableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Forgejo organization action variable resource.",

		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				Description: "Name of the organization.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
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
func (r *organizationActionVariableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *organizationActionVariableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer un(trace(ctx, "Create organization action variable resource"))

	var data organizationActionVariableResourceModel

	// Read Terraform plan data into model
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Create organization action variable", map[string]any{
		"org":   data.Organization.ValueString(),
		"name":  data.Name.ValueString(),
		"value": data.Value.ValueString(),
	})

	// Generate API request body from plan
	opts := forgejo.CreateVariableOption{
		Name: data.Name.ValueString(),
		Data: data.Value.ValueString(),
	}

	// Validate API request body
	err := opts.Validate()
	if err != nil {
		resp.Diagnostics.AddError("Input validation error", err.Error())

		return
	}

	// Use Forgejo client to create new organization action variable
	res, err := r.client.CreateOrgActionVariable(
		data.Organization.ValueString(),
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
					"Organization with name %s not found: %s",
					data.Organization.String(),
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
		resp.Diagnostics.AddError("Unable to create organization action variable", msg)

		return
	}

	// Save data into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *organizationActionVariableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer un(trace(ctx, "Read organization action variable resource"))

	var data organizationActionVariableResourceModel

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Read organization action variable", map[string]any{
		"org":  data.Organization.ValueString(),
		"name": data.Name.ValueString(),
	})

	// Use Forgejo client to get organization action variable
	variable, res, err := r.client.GetOrgActionVariable(
		data.Organization.ValueString(),
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
					"Organization action variable with org %s and name %s not found: %s",
					data.Organization.String(),
					data.Name.String(),
					err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to read organization action variable", msg)

		return
	}

	data.Name = types.StringValue(variable.Name)
	data.Value = types.StringValue(variable.Data)

	// Save data into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *organizationActionVariableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer un(trace(ctx, "Update organization action variable resource"))

	var data organizationActionVariableResourceModel

	// Read Terraform plan data into model
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Update organization action variable", map[string]any{
		"org":   data.Organization.ValueString(),
		"name":  data.Name.ValueString(),
		"value": data.Value.ValueString(),
	})

	// Generate API request body from plan
	opts := forgejo.CreateVariableOption{
		Name: data.Name.ValueString(),
		Data: data.Value.ValueString(),
	}

	// Validate API request body
	err := opts.Validate()
	if err != nil {
		resp.Diagnostics.AddError("Input validation error", err.Error())

		return
	}

	// Use Forgejo client to update organization action variable
	res, err := r.client.UpdateOrgActionVariable(
		data.Organization.ValueString(),
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
					"Organization with name %s not found: %s",
					data.Organization.String(),
					err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to update organization action variable", msg)

		return
	}

	// Save data into Terraform state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *organizationActionVariableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	defer un(trace(ctx, "Delete organization action variable resource"))

	var data organizationActionVariableResourceModel

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Delete organization action variable", map[string]any{
		"org":  data.Organization.ValueString(),
		"name": data.Name.ValueString(),
	})

	// Use Forgejo client to delete organization action variable
	res, err := r.client.DeleteOrgActionVariable(
		data.Organization.ValueString(),
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
					"Organization action variable with org %s and name %s not found: %s",
					data.Organization.String(),
					data.Name.String(),
					err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to delete organization action variable", msg)

		return
	}
}

// ImportState imports the resource state from Terraform.
func (r *organizationActionVariableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	defer un(trace(ctx, "Import organization action variable resource"))

	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected format: organization:variable_name, got: %s", req.ID),
		)
		return
	}

	org := parts[0]
	name := parts[1]

	tflog.Info(ctx, "Importing organization action variable", map[string]any{
		"org":  org,
		"name": name,
	})

	// Use Forgejo client to get variable
	variable, res, err := r.client.GetOrgActionVariable(org, name)
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
					"Organization action variable with org %s and name %s not found: %s",
					org, name, err,
				)
			default:
				msg = fmt.Sprintf("Unknown error: %s", err)
			}
		}
		resp.Diagnostics.AddError("Unable to import organization action variable", msg)

		return
	}

	var data organizationActionVariableResourceModel
	data.Organization = types.StringValue(org)
	data.Name = types.StringValue(variable.Name)
	data.Value = types.StringValue(variable.Data)

	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// NewOrganizationActionVariableResource is a helper function to simplify the provider implementation.
func NewOrganizationActionVariableResource() resource.Resource {
	return &organizationActionVariableResource{}
}
