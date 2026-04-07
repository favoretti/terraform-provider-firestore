package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

var _ provider.Provider = &FirestoreProvider{}

type FirestoreProvider struct {
	version string
}

type FirestoreProviderModel struct {
	Project                   types.String `tfsdk:"project"`
	Credentials               types.String `tfsdk:"credentials"`
	Database                  types.String `tfsdk:"database"`
	ImpersonateServiceAccount types.String `tfsdk:"impersonate_service_account"`
}

type FirestoreClient struct {
	HTTPClient *http.Client
	Project    string
	Database   string
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &FirestoreProvider{
			version: version,
		}
	}
}

func (p *FirestoreProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "firestore"
	resp.Version = p.version
}

func (p *FirestoreProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Interact with Google Cloud Firestore.",
		Attributes: map[string]schema.Attribute{
			"project": schema.StringAttribute{
				Description: "The GCP project ID. Can also be set via GOOGLE_PROJECT or GOOGLE_CLOUD_PROJECT environment variables.",
				Optional:    true,
			},
			"credentials": schema.StringAttribute{
				Description: "Service account JSON credentials. Can also be set via GOOGLE_CREDENTIALS or GOOGLE_APPLICATION_CREDENTIALS environment variables.",
				Optional:    true,
				Sensitive:   true,
			},
			"database": schema.StringAttribute{
				Description: "The Firestore database ID. Defaults to '(default)'.",
				Optional:    true,
			},
			"impersonate_service_account": schema.StringAttribute{
				Description: "The service account email to impersonate for all API calls. The caller must have the `roles/iam.serviceAccountTokenCreator` role on the target service account. Can also be set via the GOOGLE_IMPERSONATE_SERVICE_ACCOUNT environment variable.",
				Optional:    true,
			},
		},
	}
}

// resolvedProviderConfig holds the resolved values for all provider configuration
// fields after applying environment variable and HCL attribute precedence.
type resolvedProviderConfig struct {
	project                   string
	database                  string
	credentials               string
	impersonateServiceAccount string
}

// resolveProviderConfig applies the following precedence for each field,
// from lowest to highest priority: env var → HCL attribute.
//
//   - project: GOOGLE_PROJECT → GOOGLE_CLOUD_PROJECT → config.Project
//   - database: literal "(default)" → config.Database
//   - credentials: GOOGLE_CREDENTIALS → GOOGLE_APPLICATION_CREDENTIALS → config.Credentials
//   - impersonate_service_account: GOOGLE_IMPERSONATE_SERVICE_ACCOUNT → config.ImpersonateServiceAccount
func resolveProviderConfig(config FirestoreProviderModel) resolvedProviderConfig {
	project := os.Getenv("GOOGLE_PROJECT")
	if project == "" {
		project = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}
	if !config.Project.IsNull() {
		project = config.Project.ValueString()
	}

	database := "(default)"
	if !config.Database.IsNull() {
		database = config.Database.ValueString()
	}

	credentials := os.Getenv("GOOGLE_CREDENTIALS")
	if credentials == "" {
		credentials = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	}
	if !config.Credentials.IsNull() {
		credentials = config.Credentials.ValueString()
	}

	impersonateServiceAccount := os.Getenv("GOOGLE_IMPERSONATE_SERVICE_ACCOUNT")
	if !config.ImpersonateServiceAccount.IsNull() {
		impersonateServiceAccount = config.ImpersonateServiceAccount.ValueString()
	}

	return resolvedProviderConfig{
		project:                   project,
		database:                  database,
		credentials:               credentials,
		impersonateServiceAccount: impersonateServiceAccount,
	}
}

func (p *FirestoreProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring Firestore provider")

	var config FirestoreProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resolved := resolveProviderConfig(config)
	project := resolved.project
	database := resolved.database
	credentials := resolved.credentials
	impersonateServiceAccount := resolved.impersonateServiceAccount

	scopes := []string{
		"https://www.googleapis.com/auth/datastore",
		"https://www.googleapis.com/auth/cloud-platform",
	}

	// Resolve base token source
	var tokenSource oauth2.TokenSource

	if credentials != "" {
		// Check if credentials is a file path or JSON content
		var credJSON []byte
		var err error
		if _, statErr := os.Stat(credentials); statErr == nil {
			credJSON, err = os.ReadFile(credentials)
			if err != nil {
				resp.Diagnostics.AddError(
					"Unable to read credentials file",
					fmt.Sprintf("Error reading credentials file: %s", err),
				)
				return
			}
		} else {
			credJSON = []byte(credentials)
		}

		// Extract project from credentials if not set
		if project == "" {
			var credData map[string]interface{}
			if jsonErr := json.Unmarshal(credJSON, &credData); jsonErr == nil {
				if p, ok := credData["project_id"].(string); ok {
					project = p
				}
			}
		}

		creds, err := google.CredentialsFromJSON(context.Background(), credJSON, scopes...)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to create credentials",
				fmt.Sprintf("Error creating credentials from JSON: %s", err),
			)
			return
		}

		tokenSource = creds.TokenSource
	} else {
		// Use Application Default Credentials
		creds, err := google.FindDefaultCredentials(context.Background(), scopes...)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to find default credentials",
				fmt.Sprintf("Error finding default credentials: %s. Please set GOOGLE_APPLICATION_CREDENTIALS or configure credentials in the provider.", err),
			)
			return
		}

		if project == "" && creds.ProjectID != "" {
			project = creds.ProjectID
		}

		tokenSource = creds.TokenSource
	}

	// Warn when impersonation is requested but no explicit credentials were provided.
	// The impersonation call will use ADC, which may fail non-obviously if ADC is not
	// authorized to generate tokens for the target service account.
	if impersonateServiceAccount != "" && credentials == "" {
		resp.Diagnostics.AddWarning(
			"Impersonation without explicit credentials",
			fmt.Sprintf(
				"impersonate_service_account is set to %q but no credentials were configured. "+
					"Impersonation will use Application Default Credentials. "+
					"If ADC does not have the roles/iam.serviceAccountTokenCreator role on %q, "+
					"all API calls will fail. Set the credentials attribute or GOOGLE_CREDENTIALS "+
					"environment variable to suppress this warning.",
				impersonateServiceAccount, impersonateServiceAccount,
			),
		)
	}

	// Wrap token source with impersonation if configured
	if impersonateServiceAccount != "" {
		impersonateTS, err := impersonate.CredentialsTokenSource(context.Background(), impersonate.CredentialsConfig{
			TargetPrincipal: impersonateServiceAccount,
			Scopes:          scopes,
		}, option.WithTokenSource(tokenSource))
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to create impersonation credentials",
				fmt.Sprintf("Error configuring service account impersonation for %q: %s", impersonateServiceAccount, err),
			)
			return
		}
		tokenSource = impersonateTS
	}

	// Create HTTP client with authentication
	httpClient, _, err := transport.NewHTTPClient(context.Background(), option.WithTokenSource(tokenSource))
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create HTTP client",
			fmt.Sprintf("Error creating HTTP client: %s", err),
		)
		return
	}

	if project == "" {
		resp.Diagnostics.AddError(
			"Missing project",
			"The provider could not determine the GCP project. Set the 'project' attribute or GOOGLE_PROJECT environment variable.",
		)
		return
	}

	client := &FirestoreClient{
		HTTPClient: httpClient,
		Project:    project,
		Database:   database,
	}

	tflog.Info(ctx, "Configured Firestore client", map[string]interface{}{
		"project":  project,
		"database": database,
	})

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *FirestoreProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewDocumentResource,
	}
}

func (p *FirestoreProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewDocumentDataSource,
		NewDocumentsDataSource,
	}
}
