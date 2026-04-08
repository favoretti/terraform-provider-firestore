package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &DocumentsDataSource{}

type DocumentsDataSource struct {
	client *FirestoreClient
}

type DocumentsDataSourceModel struct {
	Project      types.String `tfsdk:"project"`
	Database     types.String `tfsdk:"database"`
	Collection   types.String `tfsdk:"collection"`
	Where        types.List   `tfsdk:"where"`
	OrderBy      types.List   `tfsdk:"order_by"`
	Limit        types.Int64  `tfsdk:"limit"`
	Select       types.List   `tfsdk:"select"`
	MapKey       types.String `tfsdk:"map_key"`
	Documents    types.List   `tfsdk:"documents"`
	DocumentsMap types.Map    `tfsdk:"documents_map"`
}

type WhereCondition struct {
	Field    types.String `tfsdk:"field"`
	Operator types.String `tfsdk:"operator"`
	Value    types.String `tfsdk:"value"`
}

type OrderByCondition struct {
	Field     types.String `tfsdk:"field"`
	Direction types.String `tfsdk:"direction"`
}

type DocumentResult struct {
	DocumentID types.String `tfsdk:"document_id"`
	Fields     types.String `tfsdk:"fields"`
	FieldsMap  types.Map    `tfsdk:"fields_map"`
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
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"limit": schema.Int64Attribute{
				Description: "Maximum number of documents to return.",
				Optional:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"select": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "List of field paths to return. If omitted, all fields are returned.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
			"map_key": schema.StringAttribute{
				Optional:    true,
				Description: "Field name to use as the key for documents_map. Defaults to document_id. The field must exist and have a unique, non-empty value in every returned document.",
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
						"fields_map": schema.MapAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "Top-level fields serialized as strings. Complex values (maps, arrays, geopoints) are JSON-encoded.",
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
			"documents_map": schema.MapNestedAttribute{
				Description: "Documents indexed by document_id, for use with for_each.",
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
						"fields_map": schema.MapAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "Top-level fields serialized as strings. Complex values (maps, arrays, geopoints) are JSON-encoded.",
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
			"where": schema.ListNestedAttribute{
				Description: "Filter conditions for the query. Multiple entries are combined with AND.",
				Optional:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"field": schema.StringAttribute{
							Description: "The field path to filter on.",
							Required:    true,
						},
						"operator": schema.StringAttribute{
							Description: "The operator (EQUAL, NOT_EQUAL, LESS_THAN, LESS_THAN_OR_EQUAL, GREATER_THAN, GREATER_THAN_OR_EQUAL, ARRAY_CONTAINS, IN, ARRAY_CONTAINS_ANY, NOT_IN).",
							Required:    true,
							Validators: []validator.String{
								stringvalidator.OneOf(
									"EQUAL",
									"NOT_EQUAL",
									"LESS_THAN",
									"LESS_THAN_OR_EQUAL",
									"GREATER_THAN",
									"GREATER_THAN_OR_EQUAL",
									"ARRAY_CONTAINS",
									"IN",
									"ARRAY_CONTAINS_ANY",
									"NOT_IN",
								),
							},
						},
						"value": schema.StringAttribute{
							Description: "The value to compare against. Plain strings can be passed as-is. Use jsonencode() for booleans, numbers, arrays, or objects.",
							Required:    true,
						},
					},
				},
			},
			"order_by": schema.ListNestedAttribute{
				Description: "Ordering for the query results.",
				Optional:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"field": schema.StringAttribute{
							Description: "The field path to order by.",
							Required:    true,
						},
						"direction": schema.StringAttribute{
							Description: "The direction (ASCENDING or DESCENDING).",
							Optional:    true,
							Validators: []validator.String{
								stringvalidator.OneOf("ASCENDING", "DESCENDING"),
							},
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

	var whereConditions []WhereCondition
	if !data.Where.IsNull() {
		resp.Diagnostics.Append(data.Where.ElementsAs(ctx, &whereConditions, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	var orderByConditions []OrderByCondition
	if !data.OrderBy.IsNull() {
		resp.Diagnostics.Append(data.OrderBy.ElementsAs(ctx, &orderByConditions, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	var selectFields []string
	if !data.Select.IsNull() && !data.Select.IsUnknown() {
		resp.Diagnostics.Append(data.Select.ElementsAs(ctx, &selectFields, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	hasFilters := len(whereConditions) > 0 || len(orderByConditions) > 0 || !data.Limit.IsNull()

	var documents []DocumentResult
	var diags diag.Diagnostics

	if hasFilters {
		documents, diags = d.runStructuredQuery(ctx, project, database, data.Collection.ValueString(),
			whereConditions, orderByConditions, data.Limit, selectFields)
	} else {
		documents, diags = d.listDocuments(ctx, project, database, data.Collection.ValueString(), selectFields)
	}

	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	docObjects := make([]attr.Value, len(documents))
	for i, doc := range documents {
		docObjects[i] = types.ObjectValueMust(
			map[string]attr.Type{
				"document_id": types.StringType,
				"fields":      types.StringType,
				"fields_map":  types.MapType{ElemType: types.StringType},
				"create_time": types.StringType,
				"update_time": types.StringType,
			},
			map[string]attr.Value{
				"document_id": doc.DocumentID,
				"fields":      doc.Fields,
				"fields_map":  doc.FieldsMap,
				"create_time": doc.CreateTime,
				"update_time": doc.UpdateTime,
			},
		)
	}

	docObjAttrTypes := map[string]attr.Type{
		"document_id": types.StringType,
		"fields":      types.StringType,
		"fields_map":  types.MapType{ElemType: types.StringType},
		"create_time": types.StringType,
		"update_time": types.StringType,
	}

	data.Documents = types.ListValueMust(
		types.ObjectType{AttrTypes: docObjAttrTypes},
		docObjects,
	)

	mapElems := make(map[string]attr.Value, len(documents))

	useMapKey := !data.MapKey.IsNull() && data.MapKey.ValueString() != ""
	var mapKeyField string
	if useMapKey {
		mapKeyField = data.MapKey.ValueString()
	}
	seenKeys := make(map[string]string, len(documents))

	for _, doc := range documents {
		var mapKey string
		if useMapKey {
			fieldsMapElems := doc.FieldsMap.Elements()
			keyAttr, exists := fieldsMapElems[mapKeyField]
			if !exists {
				resp.Diagnostics.AddError(
					"Missing map_key field",
					fmt.Sprintf("Document %s has no value for map_key field %q", doc.DocumentID.ValueString(), mapKeyField),
				)
				return
			}
			keyVal, ok := keyAttr.(types.String)
			if !ok || keyVal.ValueString() == "" {
				resp.Diagnostics.AddError(
					"Empty map_key value",
					fmt.Sprintf("Document %s has an empty value for map_key field %q", doc.DocumentID.ValueString(), mapKeyField),
				)
				return
			}
			mapKey = keyVal.ValueString()
		} else {
			mapKey = doc.DocumentID.ValueString()
		}

		if firstDoc, duplicate := seenKeys[mapKey]; duplicate {
			resp.Diagnostics.AddError(
				"Duplicate map_key value",
				fmt.Sprintf("Duplicate map_key value %q found in documents %s and %s", mapKey, firstDoc, doc.DocumentID.ValueString()),
			)
			return
		}
		seenKeys[mapKey] = doc.DocumentID.ValueString()

		mapElems[mapKey] = types.ObjectValueMust(
			docObjAttrTypes,
			map[string]attr.Value{
				"document_id": doc.DocumentID,
				"fields":      doc.Fields,
				"fields_map":  doc.FieldsMap,
				"create_time": doc.CreateTime,
				"update_time": doc.UpdateTime,
			},
		)
	}
	data.DocumentsMap = types.MapValueMust(types.ObjectType{AttrTypes: docObjAttrTypes}, mapElems)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *DocumentsDataSource) listDocuments(ctx context.Context, project, database, collection string, selectFields []string) ([]DocumentResult, diag.Diagnostics) {
	var diags diag.Diagnostics
	const (
		pageSize = 300
		maxPages = 100
	)

	baseURL := fmt.Sprintf("https://firestore.googleapis.com/v1/projects/%s/databases/%s/documents/%s",
		project, database, collection)

	var allDocuments []DocumentResult
	pageToken := ""

	for page := 0; page < maxPages; page++ {
		params := url.Values{}
		params.Set("pageSize", fmt.Sprintf("%d", pageSize))
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		for _, f := range selectFields {
			params.Add("mask.fieldPaths", f)
		}
		reqURL := baseURL + "?" + params.Encode()

		tflog.Debug(ctx, "Listing Firestore documents", map[string]interface{}{
			"url":  reqURL,
			"page": page,
		})

		statusCode, respBody, err := doHTTPRequest(ctx, d.client.HTTPClient, "GET", reqURL, nil, nil)
		if err != nil {
			diags.AddError("Error listing documents", err.Error())
			return nil, diags
		}

		if statusCode != http.StatusOK {
			diags.AddError("API error", fmt.Sprintf("API returned status %d: %s", statusCode, string(respBody)))
			return nil, diags
		}

		var listResp struct {
			Documents     []FirestoreDocument `json:"documents"`
			NextPageToken string              `json:"nextPageToken"`
		}
		if err := json.Unmarshal(respBody, &listResp); err != nil {
			diags.AddError("Error parsing response", err.Error())
			return nil, diags
		}

		for _, doc := range listResp.Documents {
			fieldsJSON, err := firestoreFieldsToJSON(doc.Fields)
			if err != nil {
				diags.AddError("Error converting fields", err.Error())
				return nil, diags
			}

			sm := firestoreFieldsToStringMap(doc.Fields)
			mapVals := make(map[string]attr.Value, len(sm))
			for k, v := range sm {
				mapVals[k] = types.StringValue(v)
			}
			allDocuments = append(allDocuments, DocumentResult{
				DocumentID: types.StringValue(extractDocumentID(doc.Name)),
				Fields:     types.StringValue(fieldsJSON),
				FieldsMap:  types.MapValueMust(types.StringType, mapVals),
				CreateTime: types.StringValue(doc.CreateTime),
				UpdateTime: types.StringValue(doc.UpdateTime),
			})
		}

		pageToken = listResp.NextPageToken
		if pageToken == "" {
			break
		}
	}

	if pageToken != "" {
		diags.AddWarning(
			"Results may be incomplete",
			fmt.Sprintf("Reached maximum page limit (%d pages, %d documents). Consider using where filters or limit to reduce the result set.", maxPages, len(allDocuments)),
		)
	}

	return allDocuments, diags
}

func (d *DocumentsDataSource) runStructuredQuery(ctx context.Context, project, database, collection string,
	whereConditions []WhereCondition, orderByConditions []OrderByCondition, limit types.Int64, selectFields []string) ([]DocumentResult, diag.Diagnostics) {

	var diags diag.Diagnostics

	reqURL := fmt.Sprintf("https://firestore.googleapis.com/v1/projects/%s/databases/%s/documents:runQuery",
		project, database)

	query := map[string]interface{}{
		"from": []map[string]interface{}{
			{"collectionId": collection},
		},
	}

	if len(whereConditions) > 0 {
		query["where"] = buildFirestoreWhereClause(whereConditions)
	}

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

	if !limit.IsNull() {
		query["limit"] = limit.ValueInt64()
	}

	if len(selectFields) > 0 {
		fields := make([]map[string]interface{}, len(selectFields))
		for i, f := range selectFields {
			fields[i] = map[string]interface{}{"fieldPath": f}
		}
		query["select"] = map[string]interface{}{"fields": fields}
	}

	bodyBytes, err := json.Marshal(map[string]interface{}{"structuredQuery": query})
	if err != nil {
		diags.AddError("Error marshaling query", err.Error())
		return nil, diags
	}

	tflog.Debug(ctx, "Running Firestore query", map[string]interface{}{
		"url":  reqURL,
		"body": string(bodyBytes),
	})

	statusCode, respBody, err := doHTTPRequest(ctx, d.client.HTTPClient, "POST", reqURL,
		map[string]string{"Content-Type": "application/json"}, bodyBytes)
	if err != nil {
		diags.AddError("Error running query", err.Error())
		return nil, diags
	}

	if statusCode != http.StatusOK {
		diags.AddError("API error", fmt.Sprintf("API returned status %d: %s", statusCode, string(respBody)))
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

		sm := firestoreFieldsToStringMap(result.Document.Fields)
		mapVals := make(map[string]attr.Value, len(sm))
		for k, v := range sm {
			mapVals[k] = types.StringValue(v)
		}
		documents = append(documents, DocumentResult{
			DocumentID: types.StringValue(extractDocumentID(result.Document.Name)),
			Fields:     types.StringValue(fieldsJSON),
			FieldsMap:  types.MapValueMust(types.StringType, mapVals),
			CreateTime: types.StringValue(result.Document.CreateTime),
			UpdateTime: types.StringValue(result.Document.UpdateTime),
		})
	}

	return documents, diags
}
