package plugins

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/seike460/s3ry/pkg/interfaces"
)

// SelectRequest represents an S3 Select operation request
type SelectRequest struct {
	Bucket               string            `json:"bucket"`
	Key                  string            `json:"key"`
	Query                string            `json:"query"`
	InputFormat          string            `json:"input_format"`          // CSV, JSON, Parquet
	OutputFormat         string            `json:"output_format"`         // CSV, JSON
	CompressionType      string            `json:"compression_type"`      // NONE, GZIP, BZIP2
	InputFormatOptions   map[string]string `json:"input_format_options"`  // Format-specific options
	OutputFormatOptions  map[string]string `json:"output_format_options"` // Format-specific options
	ScanRange            *ScanRange        `json:"scan_range,omitempty"`  // Byte range to scan
}

// ScanRange defines a byte range for S3 Select scanning
type ScanRange struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// SelectResponse represents the response from an S3 Select operation
type SelectResponse struct {
	Records       []string          `json:"records"`
	BytesScanned  int64             `json:"bytes_scanned"`
	BytesReturned int64             `json:"bytes_returned"`
	Stats         map[string]int64  `json:"stats"`
	Error         string            `json:"error,omitempty"`
}

// S3SelectProcessor implements SelectProcessor for native S3 Select operations
type S3SelectProcessor struct {
	client interfaces.S3Client
	logger Logger
}

// NewS3SelectProcessor creates a new S3 Select processor
func NewS3SelectProcessor(client interfaces.S3Client, logger Logger) *S3SelectProcessor {
	return &S3SelectProcessor{
		client: client,
		logger: logger,
	}
}

// Metadata returns plugin metadata
func (p *S3SelectProcessor) Metadata() PluginMetadata {
	return PluginMetadata{
		Name:        "s3-select-processor",
		Version:     "1.0.0",
		Description: "Native AWS S3 Select processor for SQL-like queries on S3 objects",
		Author:      "s3ry",
		License:     "MIT",
		Tags:        []string{"s3", "select", "sql", "query"},
	}
}

// SupportedOperations returns supported operations
func (p *S3SelectProcessor) SupportedOperations() []S3Operation {
	return []S3Operation{OperationSelect}
}

// Initialize initializes the plugin
func (p *S3SelectProcessor) Initialize(config map[string]interface{}) error {
	p.logger.Info("S3 Select processor initialized")
	return nil
}

// Execute executes the S3 Select operation
func (p *S3SelectProcessor) Execute(ctx OperationContext, args map[string]interface{}) (*OperationResult, error) {
	// Extract select request from args
	request, err := p.parseSelectRequest(args)
	if err != nil {
		return nil, fmt.Errorf("invalid select request: %w", err)
	}

	// Execute S3 Select
	response, err := p.executeSelect(ctx.Context, request)
	if err != nil {
		return &OperationResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &OperationResult{
		Success:        true,
		Message:        fmt.Sprintf("S3 Select completed - %d records, %d bytes scanned", len(response.Records), response.BytesScanned),
		Data:           response,
		BytesTotal:     response.BytesScanned,
		BytesProcessed: response.BytesReturned,
	}, nil
}

// ProcessSelect implements SelectProcessor interface
func (p *S3SelectProcessor) ProcessSelect(ctx OperationContext, query string, format string) (*OperationResult, error) {
	args := map[string]interface{}{
		"bucket":        ctx.Bucket,
		"key":           ctx.Key,
		"query":         query,
		"input_format":  format,
		"output_format": "JSON",
	}
	
	return p.Execute(ctx, args)
}

// Cleanup cleans up resources
func (p *S3SelectProcessor) Cleanup() error {
	p.logger.Info("S3 Select processor cleaned up")
	return nil
}

// executeSelect performs the actual S3 Select operation
func (p *S3SelectProcessor) executeSelect(ctx context.Context, request *SelectRequest) (*SelectResponse, error) {
	// Build S3 Select input
	input := &s3.SelectObjectContentInput{
		Bucket:     aws.String(request.Bucket),
		Key:        aws.String(request.Key),
		Expression: aws.String(request.Query),
	}

	// Set input serialization
	inputSerialization, err := p.buildInputSerialization(request)
	if err != nil {
		return nil, fmt.Errorf("failed to build input serialization: %w", err)
	}
	input.InputSerialization = inputSerialization

	// Set output serialization
	outputSerialization, err := p.buildOutputSerialization(request)
	if err != nil {
		return nil, fmt.Errorf("failed to build output serialization: %w", err)
	}
	input.OutputSerialization = outputSerialization

	// Set scan range if specified
	if request.ScanRange != nil {
		input.ScanRange = &s3.ScanRange{
			Start: aws.Int64(request.ScanRange.Start),
			End:   aws.Int64(request.ScanRange.End),
		}
	}

	// Execute S3 Select
	result, err := p.client.S3().SelectObjectContentWithContext(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("S3 Select failed: %w", err)
	}
	defer result.EventStream.Close()

	// Process results
	response := &SelectResponse{
		Records: make([]string, 0),
		Stats:   make(map[string]int64),
	}

	for event := range result.EventStream.Events() {
		switch e := event.(type) {
		case *s3.RecordsEvent:
			if e.Payload != nil {
				record := string(e.Payload)
				response.Records = append(response.Records, record)
				response.BytesReturned += int64(len(record))
			}
		case *s3.StatsEvent:
			if e.Details != nil {
				if e.Details.BytesScanned != nil {
					response.BytesScanned = *e.Details.BytesScanned
					response.Stats["bytes_scanned"] = *e.Details.BytesScanned
				}
				if e.Details.BytesProcessed != nil {
					response.Stats["bytes_processed"] = *e.Details.BytesProcessed
				}
				if e.Details.BytesReturned != nil {
					response.Stats["bytes_returned"] = *e.Details.BytesReturned
				}
			}
		case *s3.EndEvent:
			// Select completed successfully
		case *s3.ProgressEvent:
			if e.Details != nil {
				// Progress event received but no callback to invoke
				// Could be extended to support progress callbacks in the future
			}
		}
	}

	if err := result.EventStream.Err(); err != nil {
		return nil, fmt.Errorf("S3 Select stream error: %w", err)
	}

	return response, nil
}

// buildInputSerialization builds input serialization config
func (p *S3SelectProcessor) buildInputSerialization(request *SelectRequest) (*s3.InputSerialization, error) {
	input := &s3.InputSerialization{}

	// Set compression type
	if request.CompressionType != "" {
		input.CompressionType = aws.String(strings.ToUpper(request.CompressionType))
	}

	// Set format-specific configuration
	switch strings.ToUpper(request.InputFormat) {
	case "CSV":
		csv := &s3.CSVInput{}
		if options := request.InputFormatOptions; options != nil {
			if delimiter, ok := options["delimiter"]; ok && delimiter != "" {
				csv.FieldDelimiter = aws.String(delimiter)
			}
			if quote, ok := options["quote"]; ok && quote != "" {
				csv.QuoteCharacter = aws.String(quote)
			}
			if escape, ok := options["escape"]; ok && escape != "" {
				csv.QuoteEscapeCharacter = aws.String(escape)
			}
			if comments, ok := options["comments"]; ok && comments != "" {
				csv.Comments = aws.String(comments)
			}
			if header, ok := options["header"]; ok {
				csv.FileHeaderInfo = aws.String(strings.ToUpper(header))
			}
		}
		input.CSV = csv

	case "JSON":
		json := &s3.JSONInput{}
		if options := request.InputFormatOptions; options != nil {
			if jsonType, ok := options["type"]; ok {
				json.Type = aws.String(strings.ToUpper(jsonType))
			}
		}
		input.JSON = json

	case "PARQUET":
		input.Parquet = &s3.ParquetInput{}

	default:
		return nil, fmt.Errorf("unsupported input format: %s", request.InputFormat)
	}

	return input, nil
}

// buildOutputSerialization builds output serialization config
func (p *S3SelectProcessor) buildOutputSerialization(request *SelectRequest) (*s3.OutputSerialization, error) {
	output := &s3.OutputSerialization{}

	switch strings.ToUpper(request.OutputFormat) {
	case "CSV":
		csv := &s3.CSVOutput{}
		if options := request.OutputFormatOptions; options != nil {
			if delimiter, ok := options["delimiter"]; ok && delimiter != "" {
				csv.FieldDelimiter = aws.String(delimiter)
			}
			if quote, ok := options["quote"]; ok && quote != "" {
				csv.QuoteCharacter = aws.String(quote)
			}
			if escape, ok := options["escape"]; ok && escape != "" {
				csv.QuoteEscapeCharacter = aws.String(escape)
			}
		}
		output.CSV = csv

	case "JSON":
		json := &s3.JSONOutput{}
		if options := request.OutputFormatOptions; options != nil {
			if recordDelimiter, ok := options["record_delimiter"]; ok && recordDelimiter != "" {
				json.RecordDelimiter = aws.String(recordDelimiter)
			}
		}
		output.JSON = json

	default:
		return nil, fmt.Errorf("unsupported output format: %s", request.OutputFormat)
	}

	return output, nil
}

// parseSelectRequest parses args into a SelectRequest
func (p *S3SelectProcessor) parseSelectRequest(args map[string]interface{}) (*SelectRequest, error) {
	request := &SelectRequest{
		InputFormatOptions:  make(map[string]string),
		OutputFormatOptions: make(map[string]string),
	}

	// Required fields
	if bucket, ok := args["bucket"].(string); ok {
		request.Bucket = bucket
	} else {
		return nil, fmt.Errorf("bucket is required")
	}

	if key, ok := args["key"].(string); ok {
		request.Key = key
	} else {
		return nil, fmt.Errorf("key is required")
	}

	if query, ok := args["query"].(string); ok {
		request.Query = query
	} else {
		return nil, fmt.Errorf("query is required")
	}

	// Optional fields with defaults
	if inputFormat, ok := args["input_format"].(string); ok {
		request.InputFormat = inputFormat
	} else {
		request.InputFormat = "CSV"
	}

	if outputFormat, ok := args["output_format"].(string); ok {
		request.OutputFormat = outputFormat
	} else {
		request.OutputFormat = "JSON"
	}

	if compressionType, ok := args["compression_type"].(string); ok {
		request.CompressionType = compressionType
	}

	// Parse format options
	if inputOptions, ok := args["input_format_options"].(map[string]interface{}); ok {
		for k, v := range inputOptions {
			if str, ok := v.(string); ok {
				request.InputFormatOptions[k] = str
			}
		}
	}

	if outputOptions, ok := args["output_format_options"].(map[string]interface{}); ok {
		for k, v := range outputOptions {
			if str, ok := v.(string); ok {
				request.OutputFormatOptions[k] = str
			}
		}
	}

	// Parse scan range
	if scanRange, ok := args["scan_range"].(map[string]interface{}); ok {
		if start, ok := scanRange["start"].(int64); ok {
			if end, ok := scanRange["end"].(int64); ok {
				request.ScanRange = &ScanRange{
					Start: start,
					End:   end,
				}
			}
		}
	}

	return request, nil
}