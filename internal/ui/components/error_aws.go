package components

import (
	"fmt"
	"strings"
)

// AddAWSError adds an AWS-specific error with intelligent suggestions
func (e *ErrorDisplay) AddAWSError(err error) {
	if err == nil {
		return
	}

	errStr := err.Error()
	var title, message, suggestion string
	level := ErrorLevelError
	recoverable := true

	// Intelligent error categorization and suggestions
	switch {
	case strings.Contains(errStr, "NoCredentialsErr") || strings.Contains(errStr, "no credentials"):
		title = "AWS Credentials Not Found"
		message = "Unable to locate valid AWS credentials for authentication."
		suggestion = "💡 Run 'aws configure' or set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables"

	case strings.Contains(errStr, "InvalidAccessKeyId"):
		title = "Invalid AWS Access Key"
		message = "The provided AWS access key ID is not valid."
		suggestion = "💡 Check your access key ID and run 'aws configure' to update credentials"

	case strings.Contains(errStr, "SignatureDoesNotMatch"):
		title = "Invalid AWS Secret Key"
		message = "The AWS secret access key doesn't match the access key ID."
		suggestion = "💡 Verify your secret access key and run 'aws configure' to update credentials"

	case strings.Contains(errStr, "TokenRefreshRequired"):
		title = "AWS Session Expired"
		message = "Your AWS session token has expired and needs to be refreshed."
		suggestion = "💡 Refresh your AWS session or re-run 'aws configure' if using temporary credentials"

	case strings.Contains(errStr, "RequestTimeTooSkewed"):
		title = "System Clock Incorrect"
		message = "Your system clock is not synchronized with AWS servers."
		suggestion = "💡 Synchronize your system time and try again"

	case strings.Contains(errStr, "AccessDenied"):
		title = "AWS Access Denied"
		message = "You don't have permission to perform this operation."
		suggestion = "💡 Check your IAM permissions for S3 access (ListBucket, GetObject, PutObject, DeleteObject)"

	case strings.Contains(errStr, "NoSuchBucket"):
		title = "S3 Bucket Not Found"
		message = "The specified S3 bucket does not exist or is not accessible."
		suggestion = "💡 Verify the bucket name and your access permissions to this bucket"

	case strings.Contains(errStr, "NoSuchKey"):
		title = "S3 Object Not Found"
		message = "The specified S3 object does not exist."
		suggestion = "💡 Check the object key and ensure it exists in the bucket"

	case strings.Contains(errStr, "BucketNotEmpty"):
		title = "S3 Bucket Not Empty"
		message = "Cannot delete a bucket that contains objects."
		suggestion = "💡 Delete all objects in the bucket first, then try deleting the bucket again"

	case strings.Contains(errStr, "network") || strings.Contains(errStr, "timeout") || strings.Contains(errStr, "connection"):
		title = "Network Connection Error"
		message = "Unable to connect to AWS services."
		suggestion = "🌐 Check your internet connection and try again. Consider using a different region if problems persist"

	case strings.Contains(errStr, "TooManyRequests") || strings.Contains(errStr, "RequestLimitExceeded"):
		title = "AWS Rate Limit Exceeded"
		message = "Too many requests sent to AWS in a short time."
		suggestion = "⏳ Wait a moment and try again. Consider reducing concurrent operations"
		level = ErrorLevelWarning

	default:
		title = "AWS Operation Failed"
		message = fmt.Sprintf("An AWS operation failed: %s", errStr)
		suggestion = "💡 Check AWS status page and your configuration. Contact support if the problem persists"
	}

	e.AddError(level, title, message, suggestion, errStr, recoverable)
}
