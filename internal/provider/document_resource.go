package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &DocumentResource{}
var _ resource.ResourceWithImportState = &DocumentResource{}
var _ resource.ResourceWithUpgradeState = &DocumentResource{}

type DocumentResource struct {
	client *FirestoreClient
}

type DocumentResourceModel struct {
	Project    types.String `tfsdk:"project"`
	Database   types.String `tfsdk:"database"`
	Collection types.String `tfsdk:"collection"`
	DocumentID types.String `tfsdk:"document_id"`
	Fields     types.String `tfsdk:"fields"`
	Name       types.String `tfsdk:"name"`
	CreateTime types.String `tfsdk:"create_time"`
	UpdateTime types.String `tfsdk:"update_time"`
}

func NewDocumentResource() resource.Resource {
	return &DocumentResource{}
}

func (r *DocumentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_document"
}

func (r *DocumentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     1,
		Description: "Manages a Firestore document.",
		Attributes: map[string]schema.Attribute{
			"project": schema.StringAttribute{
				Description: "The GCP project ID. Overrides the provider project.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"database": schema.StringAttribute{
				Description: "The Firestore database ID. Overrides the provider database.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"collection": schema.StringAttribute{
				Description: "The collection path (e.g., 'users' or 'users/123/orders').",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"document_id": schema.StringAttribute{
				Description: "The document ID. If not provided, one will be auto-generated.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"fields": schema.StringAttribute{
				Description: "JSON string of document fields.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "The full document resource name.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"create_time": schema.StringAttribute{
				Description: "The time the document was created.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"update_time": schema.StringAttribute{
				Description: "The time the document was last updated.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// UpgradeState migrates state from schema version 0 (no version set) to version 1.
// The structure is unchanged; the upgrade preserves all existing attribute values.
func (r *DocumentResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema: &schema.Schema{
				Attributes: map[string]schema.Attribute{
					"project": schema.StringAttribute{
						Optional: true,
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"database": schema.StringAttribute{
						Optional: true,
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"collection": schema.StringAttribute{
						Required: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"document_id": schema.StringAttribute{
						Optional: true,
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"fields":      schema.StringAttribute{Required: true},
					"name":        schema.StringAttribute{Computed: true},
					"create_time": schema.StringAttribute{Computed: true},
					"update_time": schema.StringAttribute{Computed: true},
				},
			},
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				var priorState DocumentResourceModel
				resp.Diagnostics.Append(req.State.Get(ctx, &priorState)...)
				if resp.Diagnostics.HasError() {
					return
				}
				resp.Diagnostics.Append(resp.State.Set(ctx, priorState)...)
			},
		},
	}
}

func (r *DocumentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*FirestoreClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *FirestoreClient, got: %T", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *DocumentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DocumentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project := r.client.Project
	if !data.Project.IsNull() && data.Project.ValueString() != "" {
		project = data.Project.ValueString()
	}

	database := r.client.Database
	if !data.Database.IsNull() && data.Database.ValueString() != "" {
		database = data.Database.ValueString()
	}

	firestoreFields, err := jsonToFirestoreFields(data.Fields.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid fields JSON", err.Error())
		return
	}

	baseURL := fmt.Sprintf("https://firestore.googleapis.com/v1/projects/%s/databases/%s/documents/%s",
		project, database, data.Collection.ValueString())

	var reqURL string
	if !data.DocumentID.IsNull() && data.DocumentID.ValueString() != "" {
		reqURL = fmt.Sprintf("%s?documentId=%s", baseURL, url.QueryEscape(data.DocumentID.ValueString()))
	} else {
		reqURL = baseURL
	}

	body := map[string]interface{}{"fields": firestoreFields}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request body", err.Error())
		return
	}

	tflog.Debug(ctx, "Creating Firestore document", map[string]interface{}{
		"url":  reqURL,
		"body": string(bodyBytes),
	})

	statusCode, respBody, err := doHTTPRequest(ctx, r.client.HTTPClient, "POST", reqURL,
		map[string]string{"Content-Type": "application/json"}, bodyBytes)
	if err != nil {
		resp.Diagnostics.AddError("Error creating document", err.Error())
		return
	}

	if statusCode != http.StatusOK {
		resp.Diagnostics.AddError(
			"Error creating document",
			fmt.Sprintf("API returned status %d: %s", statusCode, string(respBody)),
		)
		return
	}

	var doc FirestoreDocument
	if err := json.Unmarshal(respBody, &doc); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	docID := extractDocumentID(doc.Name)

	fieldsJSON, err := firestoreFieldsToJSON(doc.Fields)
	if err != nil {
		resp.Diagnostics.AddError("Error converting fields", err.Error())
		return
	}

	data.Project = types.StringValue(project)
	data.Database = types.StringValue(database)
	data.DocumentID = types.StringValue(docID)
	data.Name = types.StringValue(doc.Name)
	data.Fields = types.StringValue(fieldsJSON)
	data.CreateTime = types.StringValue(doc.CreateTime)
	data.UpdateTime = types.StringValue(doc.UpdateTime)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DocumentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DocumentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project := r.client.Project
	if !data.Project.IsNull() && data.Project.ValueString() != "" {
		project = data.Project.ValueString()
	}

	database := r.client.Database
	if !data.Database.IsNull() && data.Database.ValueString() != "" {
		database = data.Database.ValueString()
	}

	reqURL := fmt.Sprintf("https://firestore.googleapis.com/v1/projects/%s/databases/%s/documents/%s/%s",
		project, database, data.Collection.ValueString(), data.DocumentID.ValueString())

	tflog.Debug(ctx, "Reading Firestore document", map[string]interface{}{
		"url": reqURL,
	})

	statusCode, respBody, err := doHTTPRequest(ctx, r.client.HTTPClient, "GET", reqURL, nil, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error reading document", err.Error())
		return
	}

	if statusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if statusCode != http.StatusOK {
		resp.Diagnostics.AddError(
			"Error reading document",
			fmt.Sprintf("API returned status %d: %s", statusCode, string(respBody)),
		)
		return
	}

	var doc FirestoreDocument
	if err := json.Unmarshal(respBody, &doc); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	fieldsJSON, err := firestoreFieldsToJSON(doc.Fields)
	if err != nil {
		resp.Diagnostics.AddError("Error converting fields", err.Error())
		return
	}

	data.Project = types.StringValue(project)
	data.Database = types.StringValue(database)
	data.Name = types.StringValue(doc.Name)
	data.Fields = types.StringValue(fieldsJSON)
	data.CreateTime = types.StringValue(doc.CreateTime)
	data.UpdateTime = types.StringValue(doc.UpdateTime)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DocumentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DocumentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project := r.client.Project
	if !data.Project.IsNull() && data.Project.ValueString() != "" {
		project = data.Project.ValueString()
	}

	database := r.client.Database
	if !data.Database.IsNull() && data.Database.ValueString() != "" {
		database = data.Database.ValueString()
	}

	firestoreFields, err := jsonToFirestoreFields(data.Fields.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid fields JSON", err.Error())
		return
	}

	// Build URL with updateMask so only Terraform-managed fields are written;
	// unmanaged fields in Firestore are left untouched.
	baseURL := fmt.Sprintf("https://firestore.googleapis.com/v1/projects/%s/databases/%s/documents/%s/%s",
		project, database, data.Collection.ValueString(), data.DocumentID.ValueString())

	params := url.Values{}
	for fieldPath := range firestoreFields {
		params.Add("updateMask.fieldPaths", fieldPath)
	}
	reqURL := baseURL
	if len(params) > 0 {
		reqURL = baseURL + "?" + params.Encode()
	}

	body := map[string]interface{}{"fields": firestoreFields}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request body", err.Error())
		return
	}

	tflog.Debug(ctx, "Updating Firestore document", map[string]interface{}{
		"url":  reqURL,
		"body": string(bodyBytes),
	})

	statusCode, respBody, err := doHTTPRequest(ctx, r.client.HTTPClient, "PATCH", reqURL,
		map[string]string{"Content-Type": "application/json"}, bodyBytes)
	if err != nil {
		resp.Diagnostics.AddError("Error updating document", err.Error())
		return
	}

	if statusCode != http.StatusOK {
		resp.Diagnostics.AddError(
			"Error updating document",
			fmt.Sprintf("API returned status %d: %s", statusCode, string(respBody)),
		)
		return
	}

	var doc FirestoreDocument
	if err := json.Unmarshal(respBody, &doc); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	fieldsJSON, err := firestoreFieldsToJSON(doc.Fields)
	if err != nil {
		resp.Diagnostics.AddError("Error converting fields", err.Error())
		return
	}

	data.Project = types.StringValue(project)
	data.Database = types.StringValue(database)
	data.Name = types.StringValue(doc.Name)
	data.Fields = types.StringValue(fieldsJSON)
	data.UpdateTime = types.StringValue(doc.UpdateTime)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DocumentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DocumentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project := r.client.Project
	if !data.Project.IsNull() && data.Project.ValueString() != "" {
		project = data.Project.ValueString()
	}

	database := r.client.Database
	if !data.Database.IsNull() && data.Database.ValueString() != "" {
		database = data.Database.ValueString()
	}

	reqURL := fmt.Sprintf("https://firestore.googleapis.com/v1/projects/%s/databases/%s/documents/%s/%s",
		project, database, data.Collection.ValueString(), data.DocumentID.ValueString())

	tflog.Debug(ctx, "Deleting Firestore document", map[string]interface{}{
		"url": reqURL,
	})

	statusCode, respBody, err := doHTTPRequest(ctx, r.client.HTTPClient, "DELETE", reqURL, nil, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting document", err.Error())
		return
	}

	if statusCode != http.StatusOK && statusCode != http.StatusNotFound {
		resp.Diagnostics.AddError(
			"Error deleting document",
			fmt.Sprintf("API returned status %d: %s", statusCode, string(respBody)),
		)
	}
}

func (r *DocumentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")

	if len(parts) < 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"Import ID must be in format: project/database/collection/document_id or collection/document_id",
		)
		return
	}

	if len(parts) >= 4 {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), parts[0])...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), parts[1])...)
		collection := strings.Join(parts[2:len(parts)-1], "/")
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("collection"), collection)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("document_id"), parts[len(parts)-1])...)
	} else {
		collection := strings.Join(parts[:len(parts)-1], "/")
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("collection"), collection)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("document_id"), parts[len(parts)-1])...)
	}
}

// FirestoreDocument represents a Firestore document response.
type FirestoreDocument struct {
	Name       string                 `json:"name"`
	Fields     map[string]interface{} `json:"fields"`
	CreateTime string                 `json:"createTime"`
	UpdateTime string                 `json:"updateTime"`
}

func extractDocumentID(name string) string {
	parts := strings.Split(name, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return name
}

// jsonToFirestoreFields converts a JSON string to Firestore field format.
// Uses json.Decoder with UseNumber to preserve integer precision for values
// larger than 2^53.
func jsonToFirestoreFields(jsonStr string) (map[string]interface{}, error) {
	dec := json.NewDecoder(strings.NewReader(jsonStr))
	dec.UseNumber()

	var data map[string]interface{}
	if err := dec.Decode(&data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	fields := make(map[string]interface{})
	for k, v := range data {
		fields[k] = convertToFirestoreValue(v)
	}
	return fields, nil
}

// convertToFirestoreValue converts a Go value to Firestore value format.
// json.Number values (produced by Decoder.UseNumber) are mapped to integerValue
// when they parse as int64, and to doubleValue otherwise.
func convertToFirestoreValue(v interface{}) interface{} {
	if v == nil {
		return map[string]interface{}{"nullValue": nil}
	}

	switch val := v.(type) {
	case bool:
		return map[string]interface{}{"booleanValue": val}
	case json.Number:
		if i, err := val.Int64(); err == nil {
			return map[string]interface{}{"integerValue": fmt.Sprintf("%d", i)}
		}
		if f, err := val.Float64(); err == nil {
			return map[string]interface{}{"doubleValue": f}
		}
		return map[string]interface{}{"stringValue": val.String()}
	case float64:
		if val == float64(int64(val)) {
			return map[string]interface{}{"integerValue": fmt.Sprintf("%d", int64(val))}
		}
		return map[string]interface{}{"doubleValue": val}
	case string:
		return map[string]interface{}{"stringValue": val}
	case []interface{}:
		values := make([]interface{}, len(val))
		for i, item := range val {
			values[i] = convertToFirestoreValue(item)
		}
		return map[string]interface{}{
			"arrayValue": map[string]interface{}{
				"values": values,
			},
		}
	case map[string]interface{}:
		fields := make(map[string]interface{})
		for k, v := range val {
			fields[k] = convertToFirestoreValue(v)
		}
		return map[string]interface{}{
			"mapValue": map[string]interface{}{
				"fields": fields,
			},
		}
	default:
		return map[string]interface{}{"stringValue": fmt.Sprintf("%v", val)}
	}
}

// firestoreFieldsToJSON converts Firestore fields to a JSON string.
func firestoreFieldsToJSON(fields map[string]interface{}) (string, error) {
	data := convertFromFirestoreFields(fields)
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

func convertFromFirestoreFields(fields map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range fields {
		result[k] = convertFromFirestoreValue(v)
	}
	return result
}

// convertFromFirestoreValue converts a Firestore value to a Go value.
// Integer values are parsed with strconv.ParseInt to avoid silent data loss
// on parse failure.
func convertFromFirestoreValue(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	val, ok := v.(map[string]interface{})
	if !ok {
		return v
	}

	if _, ok := val["nullValue"]; ok {
		return nil
	}
	if bv, ok := val["booleanValue"]; ok {
		return bv
	}
	if iv, ok := val["integerValue"]; ok {
		if s, ok := iv.(string); ok {
			i, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return s
			}
			return i
		}
		return iv
	}
	if dv, ok := val["doubleValue"]; ok {
		return dv
	}
	if sv, ok := val["stringValue"]; ok {
		return sv
	}
	if av, ok := val["arrayValue"]; ok {
		if avMap, ok := av.(map[string]interface{}); ok {
			if values, ok := avMap["values"].([]interface{}); ok {
				result := make([]interface{}, len(values))
				for i, item := range values {
					result[i] = convertFromFirestoreValue(item)
				}
				return result
			}
		}
		return []interface{}{}
	}
	if mv, ok := val["mapValue"]; ok {
		if mvMap, ok := mv.(map[string]interface{}); ok {
			if fields, ok := mvMap["fields"].(map[string]interface{}); ok {
				return convertFromFirestoreFields(fields)
			}
		}
		return map[string]interface{}{}
	}
	if rv, ok := val["referenceValue"]; ok {
		return rv
	}
	if gv, ok := val["geoPointValue"]; ok {
		return gv
	}
	if tv, ok := val["timestampValue"]; ok {
		return tv
	}
	if bv, ok := val["bytesValue"]; ok {
		return bv
	}

	return val
}
