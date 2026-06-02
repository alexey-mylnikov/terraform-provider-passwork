package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure PassworkProvider satisfies the provider.Provider interface.
var _ provider.Provider = &PassworkProvider{}

// PassworkProvider is the top-level provider implementation.
type PassworkProvider struct {
	version string
}

// PassworkProviderModel maps the provider configuration schema.
type PassworkProviderModel struct {
	Host                 types.String `tfsdk:"host"`
	AccessToken          types.String `tfsdk:"access_token"`
	RefreshToken         types.String `tfsdk:"refresh_token"`
	MasterPassword       types.String `tfsdk:"master_password"`
	MasterKey            types.String `tfsdk:"master_key"`
	SkipTLSVerify        types.Bool   `tfsdk:"skip_tls_verify"`
	SessionCacheFile     types.String `tfsdk:"session_cache_file"`
	SessionEncryptionKey types.String `tfsdk:"session_encryption_key"`
}

// New returns a provider factory for the given version string.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &PassworkProvider{version: version}
	}
}

// Metadata returns the provider type name and version.
func (p *PassworkProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "passwork"
	resp.Version = p.version
}

// Schema defines the provider-level configuration attributes.
func (p *PassworkProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Terraform provider for [Passwork](https://passwork.pro) — manage vaults, folders, and password items.",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				MarkdownDescription: "Passwork instance URL, e.g. `https://passwork.example.com`. May also be set via the `PASSWORK_HOST` environment variable.",
				Optional:            true,
			},
			"access_token": schema.StringAttribute{
				MarkdownDescription: "API access token. May also be set via `PASSWORK_ACCESS_TOKEN`. On first use the provider caches refreshed tokens so subsequent runs do not require a fresh token.",
				Optional:            true,
				Sensitive:           true,
			},
			"refresh_token": schema.StringAttribute{
				MarkdownDescription: "API refresh token used to obtain a new access token when the current one expires. May also be set via `PASSWORK_REFRESH_TOKEN`.",
				Optional:            true,
				Sensitive:           true,
			},
			"master_password": schema.StringAttribute{
				MarkdownDescription: "Master password for client-side encryption. Mutually exclusive with `master_key`. May also be set via `PASSWORK_MASTER_PASSWORD`.",
				Optional:            true,
				Sensitive:           true,
			},
			"master_key": schema.StringAttribute{
				MarkdownDescription: "Pre-derived master key (base64) for client-side encryption. Mutually exclusive with `master_password`. May also be set via `PASSWORK_MASTER_KEY`.",
				Optional:            true,
				Sensitive:           true,
			},
			"skip_tls_verify": schema.BoolAttribute{
				MarkdownDescription: "Disable TLS certificate verification. Use only in development environments.",
				Optional:            true,
			},
			"session_cache_file": schema.StringAttribute{
				MarkdownDescription: "Path to the session cache file. Defaults to `.terraform/passwork_session` relative to the Terraform workspace. The provider persists refreshed tokens here so subsequent runs reuse them automatically.",
				Optional:            true,
			},
			"session_encryption_key": schema.StringAttribute{
				MarkdownDescription: "Hex-encoded AES key used to encrypt the session cache file. When omitted a random key is generated on first use and stored alongside the cache file.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

// Configure instantiates the passwork client and stores it in the provider data.
func (p *PassworkProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg PassworkProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	host := envOr(cfg.Host, "PASSWORK_HOST", "")
	accessToken := envOr(cfg.AccessToken, "PASSWORK_ACCESS_TOKEN", "")
	refreshToken := envOr(cfg.RefreshToken, "PASSWORK_REFRESH_TOKEN", "")
	masterPassword := envOr(cfg.MasterPassword, "PASSWORK_MASTER_PASSWORD", "")
	masterKey := envOr(cfg.MasterKey, "PASSWORK_MASTER_KEY", "")

	sessionFile := envOr(cfg.SessionCacheFile, "PASSWORK_SESSION_CACHE_FILE", defaultSessionFile)
	sessionKeyFile := defaultSessionKeyFile
	if !cfg.SessionEncryptionKey.IsNull() && !cfg.SessionEncryptionKey.IsUnknown() {
		// When the user provides an explicit key, skip the auto-generated key
		// file and use the provided value directly via a blank keyFile so
		// ensureSessionKey returns it from the hex literal path.
		sessionKeyFile = ""
	}

	if host == "" {
		resp.Diagnostics.AddError("Missing host", "Set `host` in the provider configuration or the PASSWORK_HOST environment variable.")
		return
	}
	if accessToken == "" && !fileExists(sessionFile) {
		resp.Diagnostics.AddError("Missing access_token", "Set `access_token` in the provider configuration or the PASSWORK_ACCESS_TOKEN environment variable (only required before the first successful session cache write).")
		return
	}
	if masterPassword != "" && masterKey != "" {
		resp.Diagnostics.AddError("Conflicting attributes", "`master_password` and `master_key` are mutually exclusive.")
		return
	}

	skipTLS := !cfg.SkipTLSVerify.IsNull() && !cfg.SkipTLSVerify.IsUnknown() && cfg.SkipTLSVerify.ValueBool()

	sessionEncryptionKey := envOr(cfg.SessionEncryptionKey, "PASSWORK_SESSION_ENCRYPTION_KEY", "")

	client, err := newClient(ctx, host, accessToken, refreshToken, masterPassword, masterKey, skipTLS, sessionFile, sessionKeyFile, sessionEncryptionKey)
	if err != nil {
		resp.Diagnostics.AddError("Failed to configure Passwork client", err.Error())
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

// Resources returns the list of resource implementations.
func (p *PassworkProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewVaultResource,
		NewFolderResource,
		NewItemResource,
	}
}

// DataSources returns the list of data source implementations.
func (p *PassworkProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewVaultDataSource,
		NewVaultTypeDataSource,
		NewFolderDataSource,
		NewItemDataSource,
	}
}

// envOr returns the attribute value when set, otherwise falls back to the
// named environment variable, and finally returns def.
func envOr(attr types.String, envVar, def string) string {
	if !attr.IsNull() && !attr.IsUnknown() && attr.ValueString() != "" {
		return attr.ValueString()
	}
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	return def
}

// fileExists returns true when path names an existing regular file.
func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}
