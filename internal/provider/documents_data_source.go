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
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &DocumentsDataSource{}

type DocumentsDataSource struct {
	client *FirestoreClient
}

type DocumentsDataSourceModel struct {
	Project        types.String `tfsdk:"project"`
	Database       types.String `tfsdk:"database"`
	Collection     types.String `tfsdk:"collection"`
	FilterOperator types.String `tfsdk:"filter_operator"`
	Where          types.List   `tfsdk:"where"`
	WhereGroup     types.List   `tfsdk:"where_group"`
	OrderBy        types.List   `tfsdk:"order_by"`
	Limit          types.Int64  `tfsdk:"limit"`
	Documents      types.List   `tfsdk:"documents"`
}

type WhereCondition struct {
	Field    types.String `tfsdk:"field"`
	Operator types.String `tfsdk:"operator"`
	Value    types.String `tfsdk:"value"`
}

type WhereGroupCondition struct {
	GroupOperator types.String `tfsdk:"group_operator"`
	Where         types.List   `tfsdk:"where"`
}

type OrderByCondition struct {
	Field     types.String `tfsdk:"field"`
	Direction types.String `tfsdk:"direction"`
}

type DocumentResult struct {
	DocumentID types.String `tfsdk:"document_id"`
	Fields     types.String `tfsdk:"fields"`
	CreateTime types.String `tfsdk:"create_time"`
	UpdateTime types.String `tfsdk:"update_time"`
}

func NewDocumentsDataSource() datasource.DataSource {
	return &DocumentsDataSource{}
}

func (d *DocumentsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_documents"
}

func (d *DocumentsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists Firestore documents in a collection.",
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
			"filter_operator": schema.StringAttribute{
				Description: "The operator used to combine multiple where conditions. Must be AND or OR. Defaults to AND.",
				Optional:    true,
			},
			"limit": schema.Int64Attribute{
				Description: "Maximum number of documents to return.",
				Optional:    true,
			},
			"documents": schema.ListNestedAttribute{
				Description: "List of documents in the collection.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"document_id": schema.StringAttribute{
							Description: "The document ID.",
							Computed:    true,
						},
						"fields": schema.StringAttribute{
							Description: "JSON string of document fields.",
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
				},
			},
		},
		Blocks: map[string]schema.Block{
			"where": schema.ListNestedBlock{
				Description: "Filter conditions for the query.",
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
							Description: "The value to compare against (JSON encoded).",
							Required:    true,
						},
					},
				},
			},
			"where_group": schema.ListNestedBlock{
				Description: "A group of filter conditions combined with their own operator. Use this to nest AND/OR logic (e.g., status = active AND (role = admin OR role = editor)).",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"group_operator": schema.StringAttribute{
							Description: "The operator used to combine conditions within this group. Must be AND or OR. Defaults to AND.",
							Optional:    true,
						},
					},
					Blocks: map[string]schema.Block{
						"where": schema.ListNestedBlock{
							Description: "Filter conditions within this group.",
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
										Description: "The value to compare against (JSON encoded).",
										Required:    true,
									},
								},
							},
						},
					},
				},
			},
			"order_by": schema.ListNestedBlock{
				Description: "Ordering for the query results.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"field": schema.StringAttribute{
							Description: "The field path to order by.",
							Required:    true,
						},
						"direction": schema.StringAttribute{
							Description: "The direction (ASCENDING or DESCENDING).",
							Optional:    true,
						},
					},
				},
			},
		},
	}
}

func (d *DocumentsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DocumentsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DocumentsDataSourceModel
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

	// Parse where conditions
	var whereConditions []WhereCondition
	if !data.Where.IsNull() {
		resp.Diagnostics.Append(data.Where.ElementsAs(ctx, &whereConditions, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Parse where_group conditions
	var whereGroups []WhereGroupCondition
	if !data.WhereGroup.IsNull() {
		resp.Diagnostics.Append(data.WhereGroup.ElementsAs(ctx, &whereGroups, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		// Parse nested where conditions within each group and validate group_operator
		for gi, group := range whereGroups {
			if !group.GroupOperator.IsNull() && group.GroupOperator.ValueString() != "" {
				op := group.GroupOperator.ValueString()
				if op != "AND" && op != "OR" {
					resp.Diagnostics.AddError(
						"Invalid group_operator",
						fmt.Sprintf("where_group[%d].group_operator must be AND or OR, got: %s", gi, op),
					)
					return
				}
			}
		}
	}

	// Parse order_by conditions
	var orderByConditions []OrderByCondition
	if !data.OrderBy.IsNull() {
		resp.Diagnostics.Append(data.OrderBy.ElementsAs(ctx, &orderByConditions, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Determine filter operator (default AND)
	filterOperator := "AND"
	if !data.FilterOperator.IsNull() && data.FilterOperator.ValueString() != "" {
		filterOperator = data.FilterOperator.ValueString()
		if filterOperator != "AND" && filterOperator != "OR" {
			resp.Diagnostics.AddError(
				"Invalid filter_operator",
				fmt.Sprintf("filter_operator must be AND or OR, got: %s", filterOperator),
			)
			return
		}
	}

	// Build structured query
	hasFilters := len(whereConditions) > 0 || len(whereGroups) > 0 || len(orderByConditions) > 0 || !data.Limit.IsNull()

	var documents []DocumentResult
	var diags diag.Diagnostics

	if hasFilters {
		documents, diags = d.runStructuredQuery(ctx, project, database, data.Collection.ValueString(),
			whereConditions, whereGroups, orderByConditions, data.Limit, filterOperator)
	} else {
		documents, diags = d.listDocuments(ctx, project, database, data.Collection.ValueString())
	}

	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert to types.List
	docObjects := make([]attr.Value, len(documents))
	for i, doc := range documents {
		docObjects[i] = types.ObjectValueMust(
			map[string]attr.Type{
				"document_id": types.StringType,
				"fields":      types.StringType,
				"create_time": types.StringType,
				"update_time": types.StringType,
			},
			map[string]attr.Value{
				"document_id": doc.DocumentID,
				"fields":      doc.Fields,
				"create_time": doc.CreateTime,
				"update_time": doc.UpdateTime,
			},
		)
	}

	data.Documents = types.ListValueMust(
		types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"document_id": types.StringType,
				"fields":      types.StringType,
				"create_time": types.StringType,
				"update_time": types.StringType,
			},
		},
		docObjects,
	)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *DocumentsDataSource) listDocuments(ctx context.Context, project, database, collection string) ([]DocumentResult, diag.Diagnostics) {
	var diags diag.Diagnostics

	reqURL := fmt.Sprintf("https://firestore.googleapis.com/v1/projects/%s/databases/%s/documents/%s",
		project, database, collection)

	tflog.Debug(ctx, "Listing Firestore documents", map[string]interface{}{
		"url": reqURL,
	})

	httpReq, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		diags.AddError("Error creating request", err.Error())
		return nil, diags
	}

	httpResp, err := d.client.HTTPClient.Do(httpReq)
	if err != nil {
		diags.AddError("Error listing documents", err.Error())
		return nil, diags
	}
	defer httpResp.Body.Close()

	respBody, _ := io.ReadAll(httpResp.Body)

	if httpResp.StatusCode != http.StatusOK {
		diags.AddError("API error", fmt.Sprintf("API returned status %d: %s", httpResp.StatusCode, string(respBody)))
		return nil, diags
	}

	var listResp struct {
		Documents []FirestoreDocument `json:"documents"`
	}
	if err := json.Unmarshal(respBody, &listResp); err != nil {
		diags.AddError("Error parsing response", err.Error())
		return nil, diags
	}

	documents := make([]DocumentResult, len(listResp.Documents))
	for i, doc := range listResp.Documents {
		fieldsJSON, err := firestoreFieldsToJSON(doc.Fields)
		if err != nil {
			diags.AddError("Error converting fields", err.Error())
			return nil, diags
		}

		documents[i] = DocumentResult{
			DocumentID: types.StringValue(extractDocumentID(doc.Name)),
			Fields:     types.StringValue(fieldsJSON),
			CreateTime: types.StringValue(doc.CreateTime),
			UpdateTime: types.StringValue(doc.UpdateTime),
		}
	}

	return documents, diags
}

func buildFieldFilter(cond WhereCondition) map[string]interface{} {
	var value interface{}
	if err := json.Unmarshal([]byte(cond.Value.ValueString()), &value); err != nil {
		value = cond.Value.ValueString()
	}
	return map[string]interface{}{
		"fieldFilter": map[string]interface{}{
			"field": map[string]interface{}{
				"fieldPath": cond.Field.ValueString(),
			},
			"op":    cond.Operator.ValueString(),
			"value": convertToFirestoreValue(value),
		},
	}
}

func (d *DocumentsDataSource) runStructuredQuery(ctx context.Context, project, database, collection string,
	whereConditions []WhereCondition, whereGroups []WhereGroupCondition, orderByConditions []OrderByCondition,
	limit types.Int64, filterOperator string) ([]DocumentResult, diag.Diagnostics) {

	var diags diag.Diagnostics

	reqURL := fmt.Sprintf("https://firestore.googleapis.com/v1/projects/%s/databases/%s/documents:runQuery",
		project, database)

	// Build structured query
	query := map[string]interface{}{
		"from": []map[string]interface{}{
			{"collectionId": collection},
		},
	}

	// Collect all top-level filters (individual where conditions + where_group composites)
	var allFilters []interface{}

	for _, cond := range whereConditions {
		allFilters = append(allFilters, buildFieldFilter(cond))
	}

	for _, group := range whereGroups {
		groupOp := "AND"
		if !group.GroupOperator.IsNull() && group.GroupOperator.ValueString() != "" {
			groupOp = group.GroupOperator.ValueString()
		}

		var groupWheres []WhereCondition
		if !group.Where.IsNull() {
			group.Where.ElementsAs(ctx, &groupWheres, false)
		}

		if len(groupWheres) == 1 {
			allFilters = append(allFilters, buildFieldFilter(groupWheres[0]))
		} else if len(groupWheres) > 1 {
			groupFilters := make([]interface{}, len(groupWheres))
			for i, cond := range groupWheres {
				groupFilters[i] = buildFieldFilter(cond)
			}
			allFilters = append(allFilters, map[string]interface{}{
				"compositeFilter": map[string]interface{}{
					"op":      groupOp,
					"filters": groupFilters,
				},
			})
		}
	}

	// Build the where clause from collected filters
	if len(allFilters) == 1 {
		query["where"] = allFilters[0]
	} else if len(allFilters) > 1 {
		query["where"] = map[string]interface{}{
			"compositeFilter": map[string]interface{}{
				"op":      filterOperator,
				"filters": allFilters,
			},
		}
	}

	// Add order by
	if len(orderByConditions) > 0 {
		orderBy := make([]interface{}, len(orderByConditions))
		for i, cond := range orderByConditions {
			direction := "ASCENDING"
			if !cond.Direction.IsNull() && cond.Direction.ValueString() != "" {
				direction = cond.Direction.ValueString()
			}
			orderBy[i] = map[string]interface{}{
				"field": map[string]interface{}{
					"fieldPath": cond.Field.ValueString(),
				},
				"direction": direction,
			}
		}
		query["orderBy"] = orderBy
	}

	// Add limit
	if !limit.IsNull() {
		query["limit"] = limit.ValueInt64()
	}

	body := map[string]interface{}{
		"structuredQuery": query,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		diags.AddError("Error marshaling query", err.Error())
		return nil, diags
	}

	tflog.Debug(ctx, "Running Firestore query", map[string]interface{}{
		"url":  reqURL,
		"body": string(bodyBytes),
	})

	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		diags.AddError("Error creating request", err.Error())
		return nil, diags
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := d.client.HTTPClient.Do(httpReq)
	if err != nil {
		diags.AddError("Error running query", err.Error())
		return nil, diags
	}
	defer httpResp.Body.Close()

	respBody, _ := io.ReadAll(httpResp.Body)

	if httpResp.StatusCode != http.StatusOK {
		diags.AddError("API error", fmt.Sprintf("API returned status %d: %s", httpResp.StatusCode, string(respBody)))
		return nil, diags
	}

	var queryResp []struct {
		Document       *FirestoreDocument `json:"document"`
		ReadTime       string             `json:"readTime"`
		SkippedResults int                `json:"skippedResults"`
	}
	if err := json.Unmarshal(respBody, &queryResp); err != nil {
		diags.AddError("Error parsing response", err.Error())
		return nil, diags
	}

	var documents []DocumentResult
	for _, result := range queryResp {
		if result.Document == nil {
			continue
		}

		fieldsJSON, err := firestoreFieldsToJSON(result.Document.Fields)
		if err != nil {
			diags.AddError("Error converting fields", err.Error())
			return nil, diags
		}

		documents = append(documents, DocumentResult{
			DocumentID: types.StringValue(extractDocumentID(result.Document.Name)),
			Fields:     types.StringValue(fieldsJSON),
			CreateTime: types.StringValue(result.Document.CreateTime),
			UpdateTime: types.StringValue(result.Document.UpdateTime),
		})
	}

	return documents, diags
}
