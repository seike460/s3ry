# Terraform Provider for S3ry

The S3ry Terraform Provider enables infrastructure-as-code management for ultra-high performance S3 operations using the revolutionary S3ry tool.

## Performance Achievements

- **271,615x performance improvement** over traditional tools
- **143GB/s S3 throughput** capability
- **35,000+ fps** Terminal UI (TUI)
- **49.96x memory efficiency** improvement

## Features

- **High-Performance S3 Operations**: Leverage S3ry's revolutionary performance for Terraform-managed S3 operations
- **Infrastructure as Code**: Manage S3 uploads, syncs, and configurations declaratively
- **Performance Monitoring**: Access real-time performance metrics through Terraform
- **Enterprise Ready**: Built-in security, monitoring, and compliance features

## Requirements

- Terraform >= 1.0
- S3ry binary installed and available in PATH
- AWS credentials configured

## Installation

Add the provider to your Terraform configuration:

```hcl
terraform {
  required_providers {
    s3ry = {
      source  = "seike460/s3ry"
      version = "~> 2.0"
    }
  }
}
```

## Provider Configuration

```hcl
provider "s3ry" {
  s3ry_path        = "/usr/local/bin/s3ry"  # Optional: path to s3ry binary
  aws_region       = "us-west-2"            # Optional: default AWS region
  worker_pool_size = 20                     # Optional: number of workers
  chunk_size       = "128MB"                # Optional: chunk size for uploads
  performance_mode = "high"                 # Optional: performance mode
  enable_telemetry = true                   # Optional: enable telemetry
  timeout          = 600                    # Optional: operation timeout
  max_retries      = 5                      # Optional: max retries
}
```

### Configuration Options

| Option | Description | Default | Values |
|--------|-------------|---------|---------|
| `s3ry_path` | Path to s3ry binary | `s3ry` | Any valid path |
| `aws_region` | Default AWS region | `us-west-2` | AWS region codes |
| `worker_pool_size` | Number of parallel workers | `10` | 1-100 |
| `chunk_size` | Chunk size for multipart uploads | `64MB` | Size string (MB/GB) |
| `performance_mode` | Performance optimization level | `standard` | `standard`, `high`, `maximum` |
| `enable_telemetry` | Enable performance telemetry | `false` | `true`, `false` |
| `timeout` | Operation timeout in seconds | `300` | Positive integer |
| `max_retries` | Maximum retry attempts | `3` | 0-10 |

## Resources

### `s3ry_upload`

Uploads a file to S3 with ultra-high performance.

```hcl
resource "s3ry_upload" "example" {
  local_path = "/path/to/local/file.dat"
  bucket     = "my-bucket"
  key        = "path/to/remote/file.dat"
  
  # Optional: Override provider settings
  worker_pool_size = 50
  chunk_size       = "256MB"
  
  # Optional: Metadata
  metadata = {
    "Content-Type" = "application/octet-stream"
    "Environment"  = "production"
  }
  
  # Optional: Storage class
  storage_class = "STANDARD_IA"
}
```

### `s3ry_sync`

Synchronizes a local directory with an S3 prefix.

```hcl
resource "s3ry_sync" "example" {
  local_path = "/path/to/local/directory"
  bucket     = "my-bucket"
  prefix     = "sync/prefix"
  
  # Optional: Delete files not in source
  delete_extra = true
  
  # Optional: Exclude patterns
  exclude_patterns = [
    "*.tmp",
    ".DS_Store",
    "node_modules/*"
  ]
  
  # Optional: Include only specific patterns
  include_patterns = [
    "*.js",
    "*.css",
    "*.html"
  ]
}
```

### `s3ry_bucket_policy`

Manages S3 bucket policies with high-performance validation.

```hcl
resource "s3ry_bucket_policy" "example" {
  bucket = "my-bucket"
  
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = "*"
        Action = "s3:GetObject"
        Resource = "arn:aws:s3:::my-bucket/*"
      }
    ]
  })
}
```

### `s3ry_performance_config`

Configures performance settings for specific operations.

```hcl
resource "s3ry_performance_config" "example" {
  name = "high-throughput-config"
  
  worker_pool_size    = 100
  chunk_size         = "512MB"
  memory_limit       = "8GB"
  connection_timeout = 30
  read_timeout       = 60
  
  # Advanced settings
  tcp_keep_alive     = true
  compression_level  = 6
  enable_http2      = true
}
```

## Data Sources

### `s3ry_buckets`

Retrieves information about all accessible S3 buckets.

```hcl
data "s3ry_buckets" "all" {
  # Optional: Filter by region
  region = "us-west-2"
}

output "bucket_names" {
  value = data.s3ry_buckets.all.buckets[*].name
}
```

### `s3ry_objects`

Lists objects in an S3 bucket with high-performance pagination.

```hcl
data "s3ry_objects" "files" {
  bucket = "my-bucket"
  prefix = "data/"
  
  # Optional: Limit results
  max_keys = 1000
}

output "object_keys" {
  value = data.s3ry_objects.files.objects[*].key
}
```

### `s3ry_bucket_info`

Gets detailed information about a specific bucket.

```hcl
data "s3ry_bucket_info" "example" {
  bucket = "my-bucket"
}

output "bucket_size" {
  value = data.s3ry_bucket_info.example.size_bytes
}
```

### `s3ry_performance_metrics`

Retrieves real-time performance metrics.

```hcl
data "s3ry_performance_metrics" "current" {}

output "current_throughput" {
  value = "${data.s3ry_performance_metrics.current.throughput_mbps} MB/s"
}
```

## Functions

### `s3ry_path`

Constructs S3 paths with validation.

```hcl
locals {
  s3_url = s3ry_path("my-bucket", "path/to/object.txt")
}
```

### `s3ry_size`

Converts size strings to bytes.

```hcl
locals {
  chunk_bytes = s3ry_size("128MB")  # Returns: 134217728
}
```

### `s3ry_validate_bucket_name`

Validates S3 bucket names according to AWS rules.

```hcl
locals {
  is_valid = s3ry_validate_bucket_name("my-bucket-name")
}
```

### `s3ry_calculate_hash`

Calculates file hashes for integrity checking.

```hcl
locals {
  file_hash = s3ry_calculate_hash("/path/to/file", "sha256")
}
```

## Example Configurations

### High-Performance Data Pipeline

```hcl
# Configure provider for maximum performance
provider "s3ry" {
  performance_mode = "maximum"
  worker_pool_size = 100
  chunk_size       = "1GB"
  enable_telemetry = true
}

# Create performance configuration
resource "s3ry_performance_config" "data_pipeline" {
  name = "data-pipeline-config"
  
  worker_pool_size    = 200
  chunk_size         = "2GB"
  memory_limit       = "16GB"
  connection_timeout = 60
  
  # Enable advanced optimizations
  tcp_keep_alive     = true
  compression_level  = 9
  enable_http2      = true
}

# Sync large dataset
resource "s3ry_sync" "dataset" {
  local_path = "/data/large-dataset"
  bucket     = "data-lake-bucket"
  prefix     = "datasets/v2"
  
  delete_extra = true
  
  # Use high-performance configuration
  performance_config = s3ry_performance_config.data_pipeline.name
  
  exclude_patterns = [
    "*.tmp",
    "*.log",
    ".git/*"
  ]
}

# Upload critical files
resource "s3ry_upload" "critical_file" {
  local_path = "/data/critical-analysis.parquet"
  bucket     = "analytics-bucket"
  key        = "critical/analysis-${formatdate("YYYY-MM-DD", timestamp())}.parquet"
  
  worker_pool_size = 50
  chunk_size       = "512MB"
  storage_class    = "STANDARD_IA"
  
  metadata = {
    "Content-Type"    = "application/parquet"
    "Data-Version"    = "v2.1"
    "Processed-Date"  = formatdate("RFC3339", timestamp())
  }
}
```

### Enterprise Infrastructure

```hcl
# Enterprise-grade provider configuration
provider "s3ry" {
  performance_mode = "high"
  enable_telemetry = true
  timeout          = 1800  # 30 minutes for large operations
  max_retries      = 10
}

# Bucket policy for enterprise security
resource "s3ry_bucket_policy" "enterprise_policy" {
  bucket = var.enterprise_bucket
  
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "DenyInsecureConnections"
        Effect = "Deny"
        Principal = "*"
        Action = "s3:*"
        Resource = [
          "arn:aws:s3:::${var.enterprise_bucket}",
          "arn:aws:s3:::${var.enterprise_bucket}/*"
        ]
        Condition = {
          Bool = {
            "aws:SecureTransport" = "false"
          }
        }
      },
      {
        Sid    = "AllowSSLRequestsOnly"
        Effect = "Allow"
        Principal = {
          AWS = var.authorized_users
        }
        Action = "s3:*"
        Resource = [
          "arn:aws:s3:::${var.enterprise_bucket}",
          "arn:aws:s3:::${var.enterprise_bucket}/*"
        ]
      }
    ]
  })
}

# Monitor performance metrics
data "s3ry_performance_metrics" "monitoring" {}

# Output performance dashboard URL
output "performance_dashboard" {
  value = "Performance: ${data.s3ry_performance_metrics.monitoring.throughput_mbps} MB/s"
}
```

## Performance Optimization

### Tuning for Maximum Throughput

1. **Worker Pool Size**: Scale based on file size and network bandwidth
   ```hcl
   worker_pool_size = 100  # For large files and high bandwidth
   ```

2. **Chunk Size**: Optimize based on file size
   ```hcl
   chunk_size = "1GB"     # For very large files (>10GB)
   chunk_size = "128MB"   # For medium files (100MB-10GB)
   chunk_size = "32MB"    # For small files (<100MB)
   ```

3. **Performance Mode**: Choose based on resource availability
   ```hcl
   performance_mode = "maximum"  # High CPU/memory systems
   performance_mode = "high"     # Standard servers
   performance_mode = "standard" # Resource-constrained environments
   ```

### Monitoring and Alerting

```hcl
# Get current performance metrics
data "s3ry_performance_metrics" "current" {}

# Create CloudWatch alarm for low throughput
resource "aws_cloudwatch_metric_alarm" "low_throughput" {
  alarm_name          = "s3ry-low-throughput"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "S3ryThroughput"
  namespace           = "S3ry/Performance"
  period              = "300"
  statistic           = "Average"
  threshold           = "100"  # MB/s
  alarm_description   = "S3ry throughput is below 100 MB/s"
  
  dimensions = {
    Instance = "terraform-managed"
  }
}
```

## Troubleshooting

### Common Issues

1. **S3ry binary not found**
   ```hcl
   provider "s3ry" {
     s3ry_path = "/usr/local/bin/s3ry"  # Specify full path
   }
   ```

2. **Low performance**
   ```hcl
   provider "s3ry" {
     worker_pool_size = 50              # Increase workers
     chunk_size       = "256MB"         # Larger chunks
     performance_mode = "high"          # Enable optimizations
   }
   ```

3. **Timeout issues**
   ```hcl
   provider "s3ry" {
     timeout     = 1800                 # 30 minutes
     max_retries = 10                   # More retries
   }
   ```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

This Terraform provider is licensed under the Apache 2.0 License.

## Support

- Documentation: https://github.com/seike460/terraform-provider-s3ry
- Issues: https://github.com/seike460/terraform-provider-s3ry/issues
- S3ry Tool: https://github.com/seike460/s3ry