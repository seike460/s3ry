// Package provider implements the S3ry Terraform Provider
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure S3ryProvider satisfies various provider interfaces
var _ provider.Provider = &S3ryProvider{}
var _ provider.ProviderWithFunctions = &S3ryProvider{}

// S3ryProvider defines the provider implementation
type S3ryProvider struct {
	version string
}

// S3ryProviderModel describes the provider data model
type S3ryProviderModel struct {
	S3ryPath        types.String `tfsdk:"s3ry_path"`
	AWSRegion       types.String `tfsdk:"aws_region"`
	WorkerPoolSize  types.Int64  `tfsdk:"worker_pool_size"`
	ChunkSize       types.String `tfsdk:"chunk_size"`
	EnableTelemetry types.Bool   `tfsdk:"enable_telemetry"`
	PerformanceMode types.String `tfsdk:"performance_mode"`
	Timeout         types.Int64  `tfsdk:"timeout"`
	MaxRetries      types.Int64  `tfsdk:"max_retries"`
}

// New creates a new S3ry provider instance
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &S3ryProvider{
			version: version,
		}
	}
}

// Metadata returns the provider type name
func (p *S3ryProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "s3ry"
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data
func (p *S3ryProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The S3ry provider enables infrastructure-as-code management for ultra-high performance S3 operations. " +
			"S3ry achieves 271,615x performance improvement over traditional tools with 143GB/s throughput and 35,000+ fps TUI.",

		Attributes: map[string]schema.Attribute{
			"s3ry_path": schema.StringAttribute{
				Description: "Path to the s3ry binary. Defaults to 's3ry' (searches PATH).",
				Optional:    true,
			},
			"aws_region": schema.StringAttribute{
				Description: "Default AWS region for S3 operations. Can be overridden by AWS_DEFAULT_REGION environment variable.",
				Optional:    true,
			},
			"worker_pool_size": schema.Int64Attribute{
				Description: "Number of concurrent workers for parallel operations. Defaults to 10. Higher values increase performance but use more resources.",
				Optional:    true,
			},
			"chunk_size": schema.StringAttribute{
				Description: "Chunk size for multipart uploads (e.g., '64MB', '128MB'). Larger chunks improve performance for large files.",
				Optional:    true,
			},
			"enable_telemetry": schema.BoolAttribute{
				Description: "Enable opt-in telemetry for performance monitoring and improvement. Defaults to false.",
				Optional:    true,
			},
			"performance_mode": schema.StringAttribute{
				Description: "Performance optimization mode: 'standard', 'high', 'maximum'. Higher modes use more system resources.",
				Optional:    true,
			},
			"timeout": schema.Int64Attribute{
				Description: "Operation timeout in seconds. Defaults to 300 (5 minutes).",
				Optional:    true,
			},
			"max_retries": schema.Int64Attribute{
				Description: "Maximum number of retries for failed operations. Defaults to 3.",
				Optional:    true,
			},
		},
	}
}

// Configure prepares a S3ry API client for data sources and resources
func (p *S3ryProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data S3ryProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set default values
	s3ryPath := "s3ry"
	if !data.S3ryPath.IsNull() && !data.S3ryPath.IsUnknown() {
		s3ryPath = data.S3ryPath.ValueString()
	}

	awsRegion := os.Getenv("AWS_DEFAULT_REGION")
	if !data.AWSRegion.IsNull() && !data.AWSRegion.IsUnknown() {
		awsRegion = data.AWSRegion.ValueString()
	}
	if awsRegion == "" {
		awsRegion = "us-west-2" // Default region
	}

	workerPoolSize := int64(10)
	if !data.WorkerPoolSize.IsNull() && !data.WorkerPoolSize.IsUnknown() {
		workerPoolSize = data.WorkerPoolSize.ValueInt64()
	}

	chunkSize := "64MB"
	if !data.ChunkSize.IsNull() && !data.ChunkSize.IsUnknown() {
		chunkSize = data.ChunkSize.ValueString()
	}

	enableTelemetry := false
	if !data.EnableTelemetry.IsNull() && !data.EnableTelemetry.IsUnknown() {
		enableTelemetry = data.EnableTelemetry.ValueBool()
	}

	performanceMode := "standard"
	if !data.PerformanceMode.IsNull() && !data.PerformanceMode.IsUnknown() {
		performanceMode = data.PerformanceMode.ValueString()
	}

	timeout := int64(300)
	if !data.Timeout.IsNull() && !data.Timeout.IsUnknown() {
		timeout = data.Timeout.ValueInt64()
	}

	maxRetries := int64(3)
	if !data.MaxRetries.IsNull() && !data.MaxRetries.IsUnknown() {
		maxRetries = data.MaxRetries.ValueInt64()
	}

	// Create S3ry client configuration
	config := &S3ryConfig{
		S3ryPath:        s3ryPath,
		AWSRegion:       awsRegion,
		WorkerPoolSize:  int(workerPoolSize),
		ChunkSize:       chunkSize,
		EnableTelemetry: enableTelemetry,
		PerformanceMode: performanceMode,
		Timeout:         int(timeout),
		MaxRetries:      int(maxRetries),
	}

	// Create and validate S3ry client
	client, err := NewS3ryClient(config)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create S3ry Client",
			"An unexpected error occurred when creating the S3ry client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"S3ry Client Error: "+err.Error(),
		)
		return
	}

	// Test S3ry binary availability
	if err := client.TestConnection(); err != nil {
		resp.Diagnostics.AddError(
			"S3ry Binary Not Available",
			"The s3ry binary could not be found or executed. "+
				"Please ensure s3ry is installed and available in PATH, or specify the full path using the 's3ry_path' provider configuration.\n\n"+
				"Error: "+err.Error(),
		)
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

// Resources defines the resources implemented in the provider
func (p *S3ryProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewS3UploadResource,
		NewS3SyncResource,
		NewS3BucketPolicyResource,
		NewS3PerformanceConfigResource,
	}
}

// DataSources defines the data sources implemented in the provider
func (p *S3ryProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewS3BucketsDataSource,
		NewS3ObjectsDataSource,
		NewS3BucketInfoDataSource,
		NewS3PerformanceMetricsDataSource,
	}
}

// Functions defines the functions implemented in the provider
func (p *S3ryProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{
		NewS3PathFunction,
		NewS3SizeFunction,
		NewS3ValidateBucketNameFunction,
		NewS3CalculateHashFunction,
	}
}

// S3ryConfig holds the configuration for the S3ry client
type S3ryConfig struct {
	S3ryPath        string
	AWSRegion       string
	WorkerPoolSize  int
	ChunkSize       string
	EnableTelemetry bool
	PerformanceMode string
	Timeout         int
	MaxRetries      int
}
