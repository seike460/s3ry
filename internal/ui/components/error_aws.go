package components

import (
	"fmt"
	"strings"
)

// awsErrorHint maps an error substring to a title, message, and suggestion.
type awsErrorHint struct {
	contains   []string
	title      string
	message    string
	suggestion string
	warning    bool
}

// awsErrorHints runs in order; the first matching substring wins.
var awsErrorHints = []awsErrorHint{
	{
		contains:   []string{"NoCredentialsErr", "no credentials"},
		title:      "AWS Credentials Not Found",
		message:    "Unable to locate valid AWS credentials for authentication.",
		suggestion: "💡 Run 'aws configure' or set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables",
	},
	{
		contains:   []string{"InvalidAccessKeyId"},
		title:      "Invalid AWS Access Key",
		message:    "The provided AWS access key ID is not valid.",
		suggestion: "💡 Check your access key ID and run 'aws configure' to update credentials",
	},
	{
		contains:   []string{"SignatureDoesNotMatch"},
		title:      "Invalid AWS Secret Key",
		message:    "The AWS secret access key doesn't match the access key ID.",
		suggestion: "💡 Verify your secret access key and run 'aws configure' to update credentials",
	},
	{
		contains:   []string{"TokenRefreshRequired"},
		title:      "AWS Session Expired",
		message:    "Your AWS session token has expired and needs to be refreshed.",
		suggestion: "💡 Refresh your AWS session or re-run 'aws configure' if using temporary credentials",
	},
	{
		contains:   []string{"RequestTimeTooSkewed"},
		title:      "System Clock Incorrect",
		message:    "Your system clock is not synchronized with AWS servers.",
		suggestion: "💡 Synchronize your system time and try again",
	},
	{
		contains:   []string{"AccessDenied"},
		title:      "AWS Access Denied",
		message:    "You don't have permission to perform this operation.",
		suggestion: "💡 Check your IAM permissions for S3 access (ListBucket, GetObject, PutObject, DeleteObject)",
	},
	{
		contains:   []string{"NoSuchBucket"},
		title:      "S3 Bucket Not Found",
		message:    "The specified S3 bucket does not exist or is not accessible.",
		suggestion: "💡 Verify the bucket name and your access permissions to this bucket",
	},
	{
		contains:   []string{"NoSuchKey"},
		title:      "S3 Object Not Found",
		message:    "The specified S3 object does not exist.",
		suggestion: "💡 Check the object key and ensure it exists in the bucket",
	},
	{
		contains:   []string{"BucketNotEmpty"},
		title:      "S3 Bucket Not Empty",
		message:    "Cannot delete a bucket that contains objects.",
		suggestion: "💡 Delete all objects in the bucket first, then try deleting the bucket again",
	},
	{
		contains:   []string{"network", "timeout", "connection"},
		title:      "Network Connection Error",
		message:    "Unable to connect to AWS services.",
		suggestion: "🌐 Check your internet connection and try again. Consider using a different region if problems persist",
	},
	{
		contains:   []string{"TooManyRequests", "RequestLimitExceeded"},
		title:      "AWS Rate Limit Exceeded",
		message:    "Too many requests sent to AWS in a short time.",
		suggestion: "⏳ Wait a moment and try again. Consider reducing concurrent operations",
		warning:    true,
	},
}

// AddAWSError adds an AWS-specific error with intelligent suggestions
func (e *ErrorDisplay) AddAWSError(err error) {
	if err == nil {
		return
	}
	errStr := err.Error()
	hint := matchAWSErrorHint(errStr)
	level := ErrorLevelError
	if hint.warning {
		level = ErrorLevelWarning
	}
	e.AddError(level, hint.title, hint.message, hint.suggestion, errStr, true)
}

// matchAWSErrorHint returns the first hint whose substrings appear in errStr,
// or a generic fallback.
func matchAWSErrorHint(errStr string) awsErrorHint {
	for _, hint := range awsErrorHints {
		for _, sub := range hint.contains {
			if strings.Contains(errStr, sub) {
				return hint
			}
		}
	}
	return awsErrorHint{
		title:      "AWS Operation Failed",
		message:    fmt.Sprintf("An AWS operation failed: %s", errStr),
		suggestion: "💡 Check AWS status page and your configuration. Contact support if the problem persists",
	}
}
