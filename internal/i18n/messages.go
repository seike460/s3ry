package i18n

import (
	"fmt"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// catalog registers a translation. Entries are static and must be valid;
// a failure here is a programmer error, so init panics.
func catalog(tag language.Tag, key, value string) {
	if err := message.SetString(tag, key, value); err != nil {
		panic(fmt.Sprintf("i18n catalog %q: %v", key, err))
	}
}

// The catalog keys are the English source strings used at call sites via
// Deps.T / Printer.Sprintf. English needs no entries: a missing key falls
// back to the key itself.
func init() {
	ja := language.Japanese

	catalog(ja, "s3ry needs an interactive terminal for the TUI. Run `s3ry --help` to see the non-interactive subcommands.", "TUI には対話型ターミナルが必要です。非対話サブコマンドは `s3ry --help` を参照してください。")

	// Navigation and footer hints
	catalog(ja, "↑↓: navigate • enter: select • r: refresh • esc: back • q: quit", "↑↓: 移動 • enter: 選択 • r: 更新 • esc: 戻る • q: 終了")
	catalog(ja, "↑↓: navigate • enter: select • r: refresh • p: preview • ?: help • s: settings • esc: back • q: quit", "↑↓: 移動 • enter: 選択 • r: 更新 • p: プレビュー • ?: ヘルプ • s: 設定 • esc: 戻る • q: 終了")
	catalog(ja, "↑↓: navigate • enter: select • r: retry • ?: help • s: settings • q: quit", "↑↓: 移動 • enter: 選択 • r: 再試行 • ?: ヘルプ • s: 設定 • q: 終了")
	catalog(ja, "↑↓: navigate • esc: back • q: quit", "↑↓: 移動 • esc: 戻る • q: 終了")
	catalog(ja, "d: download • u: upload • delete: delete • ?: help • s: settings • esc: back • q: quit", "d: ダウンロード • u: アップロード • delete: 削除 • ?: ヘルプ • s: 設定 • esc: 戻る • q: 終了")
	catalog(ja, "esc: back • q: quit", "esc: 戻る • q: 終了")

	// Bucket view
	catalog(ja, "s3ry - S3 file manager", "s3ry - S3 ファイルマネージャ")
	catalog(ja, "Select S3 Bucket", "S3 バケットを選択")
	catalog(ja, "Loading S3 buckets...", "S3 バケットを読み込み中...")
	catalog(ja, "Retrying to load S3 buckets...", "S3 バケットを再読み込み中...")
	catalog(ja, "Error Loading Buckets", "バケットの読み込みエラー")
	catalog(ja, "Failed to load S3 buckets", "S3 バケットの読み込みに失敗しました")

	// Operation view
	catalog(ja, "Select Operation", "操作を選択")
	catalog(ja, "Download files", "ファイルをダウンロード")
	catalog(ja, "Download an object from the bucket (shortcut: d)", "バケットからオブジェクトをダウンロード（ショートカット: d）")
	catalog(ja, "Upload files", "ファイルをアップロード")
	catalog(ja, "Upload a local file to the bucket (shortcut: u)", "ローカルファイルをバケットへアップロード（ショートカット: u）")
	catalog(ja, "Delete objects", "オブジェクトを削除")
	catalog(ja, "Delete an object from the bucket (shortcut: delete)", "バケットからオブジェクトを削除（ショートカット: delete）")
	catalog(ja, "Create object list", "オブジェクト一覧を作成")
	catalog(ja, "Export the bucket's object list to a local file", "バケットのオブジェクト一覧をローカルファイルにエクスポート")

	// Object view
	catalog(ja, "S3 Objects", "S3 オブジェクト")
	catalog(ja, "Select Object", "オブジェクトを選択")
	catalog(ja, "Select Object to Download", "ダウンロードするオブジェクトを選択")
	catalog(ja, "Select Object to Delete", "削除するオブジェクトを選択")
	catalog(ja, "Loading S3 objects for download...", "ダウンロード対象の S3 オブジェクトを読み込み中...")
	catalog(ja, "Loading S3 objects for delete...", "削除対象の S3 オブジェクトを読み込み中...")
	catalog(ja, "Retrying to load S3 objects...", "S3 オブジェクトを再読み込み中...")
	catalog(ja, "Error Loading Objects", "オブジェクトの読み込みエラー")
	catalog(ja, "Failed to load S3 objects", "S3 オブジェクトの読み込みに失敗しました")
	catalog(ja, "Folder marker", "フォルダマーカー")
	catalog(ja, "S3 Object Information", "S3 オブジェクト情報")
	catalog(ja, "Delete %s? [y/N]", "%s を削除しますか？ [y/N]")
	catalog(ja, "The file exists. Overwrite %s? [y/N]", "ファイルが存在します。%s を上書きしますか？ [y/N]")
	catalog(ja, "Downloading %s", "%s をダウンロード中...")
	catalog(ja, "Downloaded %s (%s)", "ダウンロードしました: %s（%s）")
	catalog(ja, "Deleting %s", "%s を削除中...")
	catalog(ja, "Deleted %s", "%s を削除しました")

	// Upload view
	catalog(ja, "Local Files", "ローカルファイル")
	catalog(ja, "Select File to Upload", "アップロードするファイルを選択")
	catalog(ja, "Scanning local files...", "ローカルファイルをスキャン中...")
	catalog(ja, "Retrying to scan local files...", "ローカルファイルを再スキャン中...")
	catalog(ja, "Error Scanning Files", "ファイルスキャンエラー")
	catalog(ja, "Failed to scan local files", "ローカルファイルのスキャンに失敗しました")
	catalog(ja, "Uploading %s", "%s をアップロード中...")
	catalog(ja, "Uploaded %s (%s)", "アップロードしました: %s（%s）")

	// List generator view
	catalog(ja, "Generating Object List", "オブジェクト一覧を生成中")
	catalog(ja, "Generating object list", "オブジェクト一覧を生成中")
	catalog(ja, "Generating object list...", "オブジェクト一覧を生成中...")
	catalog(ja, "%d objects written", "%d 件のオブジェクトを書き込みました")
	catalog(ja, "Object list created: %s (%d objects)", "オブジェクト一覧を作成しました: %s（%d 件）")

	// Settings view
	catalog(ja, "Settings", "設定")
	catalog(ja, "Settings not available", "設定を利用できません")
	catalog(ja, "Application", "アプリケーション")
	catalog(ja, "AWS Configuration", "AWS 設定")
	catalog(ja, "UI Configuration", "UI 設定")
	catalog(ja, "Performance", "パフォーマンス")
	catalog(ja, "Environment", "環境変数")
	catalog(ja, "Language:", "言語:")
	catalog(ja, "Interface language (en/ja)", "UI 言語（en/ja）")
	catalog(ja, "Profile:", "プロファイル:")
	catalog(ja, "AWS shared config profile", "AWS 共有設定のプロファイル")
	catalog(ja, "Profile override from the environment", "環境変数によるプロファイル上書き")
	catalog(ja, "Region:", "リージョン:")
	catalog(ja, "Region override from the environment", "環境変数によるリージョン上書き")
	catalog(ja, "Endpoint:", "エンドポイント:")
	catalog(ja, "Custom S3 endpoint URL, for example LocalStack", "カスタム S3 エンドポイント URL（例: LocalStack）")
	catalog(ja, "Concurrency:", "並列数:")
	catalog(ja, "Parallel S3 workers for transfers and listing", "転送と一覧取得の並列 S3 ワーカー数")
	catalog(ja, "Part size:", "パートサイズ:")
	catalog(ja, "Multipart chunk size in bytes", "マルチパートのチャンクサイズ（バイト）")
	catalog(ja, "Timeout:", "タイムアウト:")
	catalog(ja, "Timeout for each blocking S3 request", "各ブロッキング S3 リクエストのタイムアウト")
	catalog(ja, "Access key from the environment (masked)", "環境変数のアクセスキー（マスク済み）")
	catalog(ja, "Secret key from the environment (masked)", "環境変数のシークレットキー（マスク済み）")
	catalog(ja, "Empty follows the AWS SDK default chain", "空の場合は AWS SDK のデフォルトチェーンに従います")
	catalog(ja, "(not set)", "（未設定）")

	// Help view
	catalog(ja, "Help - s3ry S3 Browser", "ヘルプ - s3ry S3 ブラウザ")
	catalog(ja, "s3ry - Interactive S3 Terminal Client", "s3ry - インタラクティブ S3 ターミナルクライアント")
	catalog(ja, "Navigate your S3 buckets and objects with ease", "S3 バケットとオブジェクトを快適に操作します")
	catalog(ja, "Navigation", "ナビゲーション")
	catalog(ja, "Actions", "操作")
	catalog(ja, "Move cursor up", "カーソルを上へ移動")
	catalog(ja, "Move cursor down", "カーソルを下へ移動")
	catalog(ja, "Go to first item", "最初の項目へ移動")
	catalog(ja, "Go to last item", "最後の項目へ移動")
	catalog(ja, "Next page", "次のページ")
	catalog(ja, "Previous page", "前のページ")
	catalog(ja, "Select item", "項目を選択")
	catalog(ja, "Go back / cancel operation", "戻る / 操作をキャンセル")
	catalog(ja, "Show this help", "このヘルプを表示")
	catalog(ja, "Show settings", "設定を表示")
	catalog(ja, "Refresh current view", "現在のビューを更新")
	catalog(ja, "Quit application", "アプリケーションを終了")
	catalog(ja, "Force quit", "強制終了")
	catalog(ja, "Download", "ダウンロード")
	catalog(ja, "Download selected S3 object to local file", "選択した S3 オブジェクトをローカルファイルにダウンロード")
	catalog(ja, "Upload", "アップロード")
	catalog(ja, "Upload local file to S3 bucket", "ローカルファイルを S3 バケットへアップロード")
	catalog(ja, "Delete", "削除")
	catalog(ja, "Delete selected S3 object", "選択した S3 オブジェクトを削除")
	catalog(ja, "Generate List", "一覧を生成")
	catalog(ja, "Create a list of all objects in bucket", "バケット内の全オブジェクト一覧を作成")
	catalog(ja, "Operations", "操作")

	// Shared labels and results
	catalog(ja, "Bucket:", "バケット:")
	catalog(ja, "Key:", "キー:")
	catalog(ja, "Size:", "サイズ:")
	catalog(ja, "Modified:", "更新日時:")
	catalog(ja, "ETag:", "ETag:")
	catalog(ja, "Canceled", "キャンセルしました")
	catalog(ja, "Timed out", "タイムアウトしました")
	catalog(ja, "Press 'r' to retry, 'esc' to go back, or 'q' to quit", "'r' で再試行、'esc' で戻る、'q' で終了")
}
