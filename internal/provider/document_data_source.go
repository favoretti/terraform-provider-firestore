package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &DocumentDataSource{}

type DocumentDataSource struct {
	client *FirestoreClient
}

type DocumentDataSourceModel struct {
	Project    types.String `tfsdk:"project"`
	Database   types.String `tfsdk:"database"`
	Collection types.String `tfsdk:"collection"`
	DocumentID types.String `tfsdk:"document_id"`
	Where      types.List   `tfsdk:"where"`
	Fields     types.String `tfsdk:"fields"`
	FieldsMap  types.Map    `tfsdk:"fields_map"`
	Name       types.String `tfsdk:"name"`
	CreateTime types.String `tfsdk:"create_time"`
	UpdateTime types.String `tfsdk:"update_time"`
}

func NewDocumentDataSource() datasource.DataSource {
	return &DocumentDataSource{}
}

func (d *DocumentDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_document"
}

func (d *DocumentDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Retrieves a single Firestore document by ID or by filter conditions.",
		Attributes: map[string]schema.Attribute{
			"project": schema.StringAttribute{
				Description: "The GCP project ID. Overrides the provider project.",
				Optional:    true,
			},
			"database": schema.StringAttribute{
				Description: "The Firestore database ID. Overrides the provider database.",
				Optional:    true,
			},
			"collection": schema.StringAttribute{
				Description: "The collection path (e.g., 'users' or 'users/123/orders').",
				Required:    true,
			},
			"document_id": schema.StringAttribute{
				Description: "The document ID to retrieve. Mutually exclusive with where blocks.",
				Optional:    true,
				Computed:    true,
			},
			"fields": schema.StringAttribute{
				Description: "JSON string of document fields.",
				Computed:    true,
			},
			"fields_map": schema.MapAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Top-level string-valued fields as a map. Non-string and nested fields are omitted.",
			},
			"name": schema.StringAttribute{
				Description: "The full document resource name.",
				Computed:    true,
			},
			"create_time": schema.StringAttribute{
				Description: "The time the document was created.",
				Computed:    true,
			},
			"update_time": schema.StringAttribute{
				Description: "The time the document was last updated.",
				Computed:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"where": schema.ListNestedBlock{
				Description: "Filter conditions for the query. Used when document_id is not specified. Multiple where blocks are combined with AND.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"field": schema.StringAttribute{
							Description: "The field path to filter on.",
							Required:    true,
						},
						"operator": schema.StringAttribute{
							Description: "The operator (EQUAL, NOT_EQUAL, LESS_THAN, LESS_THAN_OR_EQUAL, GREATER_THAN, GREATER_THAN_OR_EQUAL, ARRAY_CONTAINS, IN, ARRAY_CONTAINS_ANY, NOT_IN).",
							Required:    true,
						},
						"value": schema.StringAttribute{
							Description: "The value to compare against. Plain strings can be passed as-is. Use jsonencode() for booleans, numbers, arrays, or objects.",
							Required:    true,
						},
					},
				},
			},
		},
	}
}

func (d *DocumentDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*FirestoreClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *FirestoreClient, got: %T", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *DocumentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DocumentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project := d.client.Project
	if !data.Project.IsNull() && data.Project.ValueString() != "" {
		project = data.Project.ValueString()
	}

	database := d.client.Database
	if !data.Database.IsNull() && data.Database.ValueString() != "" {
		database = data.Database.ValueString()
	}

	hasDocumentID := !data.DocumentID.IsNull() && data.DocumentID.ValueString() != ""

	var whereConditions []WhereCondition
	if !data.Where.IsNull() {
		resp.Diagnostics.Append(data.Where.ElementsAs(ctx, &whereConditions, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	if !hasDocumentID && len(whereConditions) == 0 {
		resp.Diagnostics.AddError(
			"Invalid configuration",
			"Either document_id or at least one where block must be specified.",
		)
		return
	}

	if hasDocumentID {
		d.readByID(ctx, project, database, data.Collection.ValueString(), data.DocumentID.ValueString(), &data, resp)
	} else {
		d.readByWhere(ctx, project, database, data.Collection.ValueString(), whereConditions, &data, resp)
	}
}

func (d *DocumentDataSource) readByID(ctx context.Context, project, database, collection, documentID string, data *DocumentDataSourceModel, resp *datasource.ReadResponse) {
	reqURL := fmt.Sprintf("https://firestore.googleapis.com/v1/projects/%s/databases/%s/documents/%s/%s",
		project, database, collection, documentID)

	tflog.Debug(ctx, "Reading Firestore document by ID", map[string]interface{}{
		"url": reqURL,
	})

	httpReq, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}

	httpResp, err := d.client.HTTPClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Error reading document", err.Error())
		return
	}
	defer httpResp.Body.Close()

	respBody, _ := io.ReadAll(httpResp.Body)

	if httpResp.StatusCode == http.StatusNotFound {
		resp.Diagnostics.AddError(
			"Document not found",
			fmt.Sprintf("Document %s/%s not found in project %s, database %s", collection, documentID, project, database),
		)
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Error reading document",
			fmt.Sprintf("API returned status %d: %s", httpResp.StatusCode, string(respBody)))
		return
	}

	var doc FirestoreDocument
	if err := json.Unmarshal(respBody, &doc); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	if err := populateDocumentModel(data, &doc); err != nil {
		resp.Diagnostics.AddError("Error converting fields", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (d *DocumentDataSource) readByWhere(ctx context.Context, project, database, collection string, conditions []WhereCondition, data *DocumentDataSourceModel, resp *datasource.ReadResponse) {
	reqURL := fmt.Sprintf("https://firestore.googleapis.com/v1/projects/%s/databases/%s/documents:runQuery",
		project, database)

	query := map[string]interface{}{
		"from":  []map[string]interface{}{{"collectionId": collection}},
		"where": buildFirestoreWhereClause(conditions),
		"limit": 1,
	}

	bodyBytes, err := json.Marshal(map[string]interface{}{"structuredQuery": query})
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling query", err.Error())
		return
	}

	tflog.Debug(ctx, "Reading Firestore document by where", map[string]interface{}{
		"url":  reqURL,
		"body": string(bodyBytes),
	})

	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		resp.Diagnostics.AddError("Error creating request", err.Error())
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := d.client.HTTPClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Error querying documents", err.Error())
		return
	}
	defer httpResp.Body.Close()

	respBody, _ := io.ReadAll(httpResp.Body)

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API error",
			fmt.Sprintf("API returned status %d: %s", httpResp.StatusCode, string(respBody)))
		return
	}

	var queryResp []struct {
		Document *FirestoreDocument `json:"document"`
	}
	if err := json.Unmarshal(respBody, &queryResp); err != nil {
		resp.Diagnostics.AddError("Error parsing response", err.Error())
		return
	}

	var matched *FirestoreDocument
	for _, result := range queryResp {
		if result.Document != nil {
			matched = result.Document
			break
		}
	}

	if matched == nil {
		resp.Diagnostics.AddError(
			"Document not found",
			fmt.Sprintf("No document in collection %q matched the specified where conditions.", collection),
		)
		return
	}

	data.DocumentID = types.StringValue(extractDocumentID(matched.Name))

	if err := populateDocumentModel(data, matched); err != nil {
		resp.Diagnostics.AddError("Error converting fields", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func populateDocumentModel(data *DocumentDataSourceModel, doc *FirestoreDocument) error {
	fieldsJSON, err := firestoreFieldsToJSON(doc.Fields)
	if err != nil {
		return fmt.Errorf("converting fields: %w", err)
	}

	sm := firestoreFieldsToStringMap(doc.Fields)
	mapVals := make(map[string]attr.Value, len(sm))
	for k, v := range sm {
		mapVals[k] = types.StringValue(v)
	}

	data.Name = types.StringValue(doc.Name)
	data.Fields = types.StringValue(fieldsJSON)
	data.FieldsMap = types.MapValueMust(types.StringType, mapVals)
	data.CreateTime = types.StringValue(doc.CreateTime)
	data.UpdateTime = types.StringValue(doc.UpdateTime)
	return nil
}
