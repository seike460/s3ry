package errors

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
)

// initializeDefaultPatterns はデフォルトエラーパターンを初期化
func (h *EnhancedErrorHandler) initializeDefaultPatterns() {
	patterns := map[string]*EnhancedErrorPattern{
		// AWS S3 エラーパターン
		"NoSuchBucket": {
			Pattern:     "NoSuchBucket",
			Type:        ErrCodeValidation,
			Severity:    "medium",
			Recoverable: false,
			UserMessage: "指定されたバケットが存在しません。バケット名を確認してください。",
			HelpURL:     "https://docs.aws.amazon.com/s3/latest/userguide/create-bucket-overview.html",
			Suggestions: []string{
				"バケット名のスペルを確認してください",
				"バケットが正しいリージョンに存在するか確認してください",
				"バケットが削除されていないか確認してください",
			},
		},
		"NoSuchKey": {
			Pattern:     "NoSuchKey",
			Type:        ErrCodeValidation,
			Severity:    "medium",
			Recoverable: false,
			UserMessage: "指定されたオブジェクトが存在しません。オブジェクトキーを確認してください。",
			HelpURL:     "https://docs.aws.amazon.com/s3/latest/userguide/object-keys.html",
			Suggestions: []string{
				"オブジェクトキーのスペルを確認してください",
				"オブジェクトが削除されていないか確認してください",
				"プレフィックスが正しいか確認してください",
			},
		},
		"AccessDenied": {
			Pattern:     "AccessDenied",
			Type:        ErrCodePermission,
			Severity:    "high",
			Recoverable: false,
			UserMessage: "アクセスが拒否されました。適切な権限があることを確認してください。",
			HelpURL:     "https://docs.aws.amazon.com/s3/latest/userguide/access-control-overview.html",
			Suggestions: []string{
				"IAMポリシーを確認してください",
				"バケットポリシーを確認してください",
				"ACL設定を確認してください",
				"MFA要件がないか確認してください",
			},
		},
		"InvalidAccessKeyId": {
			Pattern:     "InvalidAccessKeyId",
			Type:        ErrCodeS3Permission,
			Severity:    "high",
			Recoverable: false,
			UserMessage: "無効なアクセスキーIDです。認証情報を確認してください。",
			HelpURL:     "https://docs.aws.amazon.com/general/latest/gr/aws-sec-cred-types.html",
			Suggestions: []string{
				"AWS認証情報を確認してください",
				"アクセスキーが有効か確認してください",
				"環境変数やプロファイル設定を確認してください",
			},
		},
		"SignatureDoesNotMatch": {
			Pattern:     "SignatureDoesNotMatch",
			Type:        ErrCodeS3Permission,
			Severity:    "high",
			Recoverable: false,
			UserMessage: "署名が一致しません。シークレットアクセスキーを確認してください。",
			HelpURL:     "https://docs.aws.amazon.com/general/latest/gr/signature-version-4.html",
			Suggestions: []string{
				"シークレットアクセスキーを確認してください",
				"システム時刻が正確か確認してください",
				"リージョン設定を確認してください",
			},
		},
		"RequestTimeout": {
			Pattern:     "RequestTimeout",
			Type:        ErrCodeTimeout,
			Severity:    "medium",
			Recoverable: true,
			UserMessage: "リクエストがタイムアウトしました。しばらく待ってから再試行してください。",
			HelpURL:     "https://docs.aws.amazon.com/s3/latest/userguide/troubleshooting.html",
			Suggestions: []string{
				"ネットワーク接続を確認してください",
				"しばらく待ってから再試行してください",
				"タイムアウト設定を調整してください",
			},
		},
		"SlowDown": {
			Pattern:     "SlowDown",
			Type:        ErrCodeTimeout,
			Severity:    "medium",
			Recoverable: true,
			UserMessage: "リクエスト頻度が高すぎます。しばらく待ってから再試行してください。",
			HelpURL:     "https://docs.aws.amazon.com/s3/latest/userguide/optimizing-performance.html",
			Suggestions: []string{
				"リクエスト頻度を下げてください",
				"指数バックオフを使用してください",
				"並列処理数を調整してください",
			},
		},
		"ServiceUnavailable": {
			Pattern:     "ServiceUnavailable",
			Type:        ErrCodeNetwork,
			Severity:    "critical",
			Recoverable: true,
			UserMessage: "S3サービスが一時的に利用できません。しばらく待ってから再試行してください。",
			HelpURL:     "https://status.aws.amazon.com/",
			Suggestions: []string{
				"AWS Service Healthを確認してください",
				"しばらく待ってから再試行してください",
				"別のリージョンを試してください",
			},
		},
		"InternalError": {
			Pattern:     "InternalError",
			Type:        ErrCodeNetwork,
			Severity:    "critical",
			Recoverable: true,
			UserMessage: "AWS内部エラーが発生しました。しばらく待ってから再試行してください。",
			HelpURL:     "https://status.aws.amazon.com/",
			Suggestions: []string{
				"しばらく待ってから再試行してください",
				"AWS Supportに連絡してください",
				"別のリージョンを試してください",
			},
		},

		// ファイルシステムエラーパターン
		"no such file or directory": {
			Pattern:     "no such file or directory",
			Type:        ErrCodeFileSystem,
			Severity:    "medium",
			Recoverable: false,
			UserMessage: "指定されたファイルまたはディレクトリが存在しません。",
			Suggestions: []string{
				"ファイルパスを確認してください",
				"ファイルが削除されていないか確認してください",
				"権限を確認してください",
			},
		},
		"permission denied": {
			Pattern:     "permission denied",
			Type:        ErrCodeFileSystem,
			Severity:    "high",
			Recoverable: false,
			UserMessage: "ファイルまたはディレクトリへのアクセス権限がありません。",
			Suggestions: []string{
				"ファイル権限を確認してください",
				"実行ユーザーを確認してください",
				"ディレクトリ権限を確認してください",
			},
		},
		"disk full": {
			Pattern:     "no space left on device",
			Type:        ErrCodeFileSystem,
			Severity:    "critical",
			Recoverable: false,
			UserMessage: "ディスク容量が不足しています。",
			Suggestions: []string{
				"ディスク容量を確認してください",
				"不要なファイルを削除してください",
				"別のディスクを使用してください",
			},
		},

		// ネットワークエラーパターン
		"connection refused": {
			Pattern:     "connection refused",
			Type:        ErrCodeNetwork,
			Severity:    "high",
			Recoverable: true,
			UserMessage: "接続が拒否されました。ネットワーク設定を確認してください。",
			Suggestions: []string{
				"ネットワーク接続を確認してください",
				"プロキシ設定を確認してください",
				"ファイアウォール設定を確認してください",
			},
		},
		"timeout": {
			Pattern:     "timeout",
			Type:        ErrCodeTimeout,
			Severity:    "medium",
			Recoverable: true,
			UserMessage: "接続がタイムアウトしました。",
			Suggestions: []string{
				"ネットワーク接続を確認してください",
				"タイムアウト設定を調整してください",
				"しばらく待ってから再試行してください",
			},
		},

		// 設定エラーパターン
		"invalid configuration": {
			Pattern:     "invalid configuration",
			Type:        ErrCodeInvalidConfig,
			Severity:    "high",
			Recoverable: false,
			UserMessage: "設定が無効です。設定ファイルを確認してください。",
			Suggestions: []string{
				"設定ファイルの構文を確認してください",
				"必須パラメータが設定されているか確認してください",
				"設定値の形式を確認してください",
			},
		},
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for pattern, config := range patterns {
		h.errorPatterns[pattern] = config
	}
}

// initializeRecoveryStrategies はリカバリ戦略を初期化
func (h *EnhancedErrorHandler) initializeRecoveryStrategies() {
	strategies := map[ErrorCode]*RecoveryStrategy{
		ErrCodeNetwork: {
			Type:       ErrCodeNetwork,
			MaxRetries: 3,
			BackoffFunc: func(attempt int) time.Duration {
				return time.Duration(attempt*attempt) * time.Second
			},
			RecoverFunc: func(ctx context.Context, err *S3ryError) (*S3ryError, bool) {
				// ネットワークエラーの場合は単純にリトライ
				return err, true
			},
			Conditions: []string{"temporary", "timeout", "connection"},
		},
		ErrCodeTimeout: {
			Type:       ErrCodeTimeout,
			MaxRetries: 5,
			BackoffFunc: func(attempt int) time.Duration {
				return time.Duration(attempt*2) * time.Second
			},
			RecoverFunc: func(ctx context.Context, err *S3ryError) (*S3ryError, bool) {
				// タイムアウトエラーの場合はより長い待機時間でリトライ
				return err, true
			},
			Conditions: []string{"timeout", "slow"},
		},
		ErrCodeS3Connection: {
			Type:       ErrCodeS3Connection,
			MaxRetries: 3,
			BackoffFunc: func(attempt int) time.Duration {
				return time.Duration(attempt) * time.Second
			},
			RecoverFunc: func(ctx context.Context, err *S3ryError) (*S3ryError, bool) {
				// S3エラーの場合は条件に応じてリトライ
				if awsCode, ok := err.Context["aws_code"].(string); ok {
					if awsCode == "InternalError" || awsCode == "ServiceUnavailable" {
						return err, true
					}
				}
				return err, false
			},
			Conditions: []string{"internal", "service_unavailable"},
		},
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for errorCode, strategy := range strategies {
		h.recoveryStrategies[errorCode] = strategy
	}
}

// initializeRetryPolicies はリトライポリシーを初期化
func (h *EnhancedErrorHandler) initializeRetryPolicies() {
	policies := map[ErrorCode]*RetryPolicy{
		ErrCodeNetwork: {
			MaxRetries:    5,
			InitialDelay:  1 * time.Second,
			MaxDelay:      30 * time.Second,
			BackoffFactor: 2.0,
			Jitter:        true,
		},
		ErrCodeTimeout: {
			MaxRetries:    3,
			InitialDelay:  2 * time.Second,
			MaxDelay:      60 * time.Second,
			BackoffFactor: 2.0,
			Jitter:        true,
		},
		ErrCodeS3Connection: {
			MaxRetries:    3,
			InitialDelay:  1 * time.Second,
			MaxDelay:      15 * time.Second,
			BackoffFactor: 1.5,
			Jitter:        false,
		},
		ErrCodeS3Permission: {
			MaxRetries:    1,
			InitialDelay:  0,
			MaxDelay:      0,
			BackoffFactor: 1.0,
			Jitter:        false,
		},
		ErrCodePermission: {
			MaxRetries:    1,
			InitialDelay:  0,
			MaxDelay:      0,
			BackoffFactor: 1.0,
			Jitter:        false,
		},
		ErrCodeValidation: {
			MaxRetries:    0,
			InitialDelay:  0,
			MaxDelay:      0,
			BackoffFactor: 1.0,
			Jitter:        false,
		},
		ErrCodeInvalidConfig: {
			MaxRetries:    0,
			InitialDelay:  0,
			MaxDelay:      0,
			BackoffFactor: 1.0,
			Jitter:        false,
		},
		ErrCodeFileSystem: {
			MaxRetries:    2,
			InitialDelay:  500 * time.Millisecond,
			MaxDelay:      5 * time.Second,
			BackoffFactor: 2.0,
			Jitter:        true,
		},
		ErrCodeInternal: {
			MaxRetries:    3,
			InitialDelay:  1 * time.Second,
			MaxDelay:      10 * time.Second,
			BackoffFactor: 2.0,
			Jitter:        true,
		},
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for errorCode, policy := range policies {
		h.retryPolicies[errorCode] = policy
	}
}

// getAWSUserMessage はAWSエラーのユーザーメッセージを取得
func (h *EnhancedErrorHandler) getAWSUserMessage(awsErr awserr.Error) string {
	code := awsErr.Code()

	userMessages := map[string]string{
		"NoSuchBucket":          "指定されたバケットが存在しません。バケット名を確認してください。",
		"NoSuchKey":             "指定されたオブジェクトが存在しません。オブジェクトキーを確認してください。",
		"AccessDenied":          "アクセスが拒否されました。適切な権限があることを確認してください。",
		"InvalidAccessKeyId":    "無効なアクセスキーIDです。認証情報を確認してください。",
		"SignatureDoesNotMatch": "署名が一致しません。シークレットアクセスキーを確認してください。",
		"RequestTimeout":        "リクエストがタイムアウトしました。しばらく待ってから再試行してください。",
		"SlowDown":              "リクエスト頻度が高すぎます。しばらく待ってから再試行してください。",
		"ServiceUnavailable":    "S3サービスが一時的に利用できません。しばらく待ってから再試行してください。",
		"InternalError":         "AWS内部エラーが発生しました。しばらく待ってから再試行してください。",
		"BucketAlreadyExists":   "バケット名が既に使用されています。別の名前を選択してください。",
		"BucketNotEmpty":        "バケットが空ではありません。オブジェクトを削除してから再試行してください。",
		"InvalidBucketName":     "無効なバケット名です。バケット命名規則を確認してください。",
		"TooManyBuckets":        "バケット数の上限に達しています。不要なバケットを削除してください。",
		"EntityTooLarge":        "オブジェクトサイズが大きすぎます。マルチパートアップロードを使用してください。",
		"InvalidPart":           "無効なパートです。マルチパートアップロードを確認してください。",
		"NoSuchUpload":          "指定されたマルチパートアップロードが存在しません。",
		"PreconditionFailed":    "前提条件が満たされていません。条件を確認してください。",
		"RequestTimeTooSkewed":  "リクエスト時刻が大きくずれています。システム時刻を確認してください。",
		"TokenRefreshRequired":  "トークンの更新が必要です。認証情報を更新してください。",
		"ExpiredToken":          "トークンが期限切れです。認証情報を更新してください。",
	}

	if msg, exists := userMessages[code]; exists {
		return msg
	}

	return fmt.Sprintf("AWS S3エラーが発生しました: %s", awsErr.Message())
}

// getAWSSuggestions はAWSエラーの提案を取得
func (h *EnhancedErrorHandler) getAWSSuggestions(awsErr awserr.Error) []string {
	code := awsErr.Code()

	suggestions := map[string][]string{
		"NoSuchBucket": {
			"バケット名のスペルを確認してください",
			"バケットが正しいリージョンに存在するか確認してください",
			"バケットが削除されていないか確認してください",
		},
		"NoSuchKey": {
			"オブジェクトキーのスペルを確認してください",
			"オブジェクトが削除されていないか確認してください",
			"プレフィックスが正しいか確認してください",
		},
		"AccessDenied": {
			"IAMポリシーを確認してください",
			"バケットポリシーを確認してください",
			"ACL設定を確認してください",
			"MFA要件がないか確認してください",
		},
		"InvalidAccessKeyId": {
			"AWS認証情報を確認してください",
			"アクセスキーが有効か確認してください",
			"環境変数やプロファイル設定を確認してください",
		},
		"SignatureDoesNotMatch": {
			"シークレットアクセスキーを確認してください",
			"システム時刻が正確か確認してください",
			"リージョン設定を確認してください",
		},
		"RequestTimeout": {
			"ネットワーク接続を確認してください",
			"しばらく待ってから再試行してください",
			"タイムアウト設定を調整してください",
		},
		"SlowDown": {
			"リクエスト頻度を下げてください",
			"指数バックオフを使用してください",
			"並列処理数を調整してください",
		},
		"ServiceUnavailable": {
			"AWS Service Healthを確認してください",
			"しばらく待ってから再試行してください",
			"別のリージョンを試してください",
		},
		"InternalError": {
			"しばらく待ってから再試行してください",
			"AWS Supportに連絡してください",
			"別のリージョンを試してください",
		},
	}

	if suggestions, exists := suggestions[code]; exists {
		return suggestions
	}

	return []string{
		"AWS S3ドキュメントを確認してください",
		"しばらく待ってから再試行してください",
		"AWS Supportに連絡してください",
	}
}

// getAWSHelpURL はAWSエラーのヘルプURLを取得
func (h *EnhancedErrorHandler) getAWSHelpURL(awsErr awserr.Error) string {
	code := awsErr.Code()

	helpURLs := map[string]string{
		"NoSuchBucket":          "https://docs.aws.amazon.com/s3/latest/userguide/create-bucket-overview.html",
		"NoSuchKey":             "https://docs.aws.amazon.com/s3/latest/userguide/object-keys.html",
		"AccessDenied":          "https://docs.aws.amazon.com/s3/latest/userguide/access-control-overview.html",
		"InvalidAccessKeyId":    "https://docs.aws.amazon.com/general/latest/gr/aws-sec-cred-types.html",
		"SignatureDoesNotMatch": "https://docs.aws.amazon.com/general/latest/gr/signature-version-4.html",
		"RequestTimeout":        "https://docs.aws.amazon.com/s3/latest/userguide/troubleshooting.html",
		"SlowDown":              "https://docs.aws.amazon.com/s3/latest/userguide/optimizing-performance.html",
		"ServiceUnavailable":    "https://status.aws.amazon.com/",
		"InternalError":         "https://status.aws.amazon.com/",
	}

	if url, exists := helpURLs[code]; exists {
		return url
	}

	return "https://docs.aws.amazon.com/s3/latest/userguide/troubleshooting.html"
}
