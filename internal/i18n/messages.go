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

func init() {
	// English messages
	catalog(language.English, "Which bucket do you use?", "Which bucket do you use?")
	catalog(language.English, "download", "download")
	catalog(language.English, "upload", "upload")
	catalog(language.English, "delete object", "delete object")
	catalog(language.English, "create object list", "create object list")
	catalog(language.English, "Searching for buckets ...", "Searching for buckets ...")
	catalog(language.English, "Searching for objects ...", "Searching for objects ...")
	catalog(language.English, "Number of objects: ", "Number of objects: ")
	catalog(language.English, "Downloading object ...", "Downloading object ...")
	catalog(language.English, "File downloaded,% s,% d bytes", "File downloaded, %s, %d bytes")
	catalog(language.English, "Uploading object ...", "Uploading object ...")
	catalog(language.English, "Uploaded file,% s", "Uploaded file, %s")
	catalog(language.English, "\"Selection Value:\" {{ .Val | red | cyan }}", "Selection Value: %s")
	catalog(language.English, "What are you doing?", "What are you doing?")
	catalog(language.English, "Which file do you upload?", "Which file do you upload?")
	catalog(language.English, "Which files do you want to delete?", "Which files do you want to delete?")
	catalog(language.English, "Which file do you want to download?", "Which file do you want to download?")
	catalog(language.English, "The file exists. Overwrite? File name:% s, [Yy] / [Nn]", "The file exists. Overwrite? File name: %s, [Yy] / [Nn]")
	catalog(language.English, "Object list created:", "Object list created: ")
	catalog(language.English, "s3ry needs an interactive terminal for the TUI. Run `s3ry --help` to see the non-interactive subcommands.", "s3ry needs an interactive terminal for the TUI. Run `s3ry --help` to see the non-interactive subcommands.")

	// Japanese messages
	catalog(language.Japanese, "Which bucket do you use?", "どのバケットを使用しますか？")
	catalog(language.Japanese, "download", "ダウンロード")
	catalog(language.Japanese, "upload", "アップロード")
	catalog(language.Japanese, "delete object", "オブジェクトを削除")
	catalog(language.Japanese, "create object list", "オブジェクトリストを作成")
	catalog(language.Japanese, "Searching for buckets ...", "バケットを検索中...")
	catalog(language.Japanese, "Searching for objects ...", "オブジェクトを検索中...")
	catalog(language.Japanese, "Number of objects: ", "オブジェクト数: ")
	catalog(language.Japanese, "Downloading object ...", "オブジェクトをダウンロード中...")
	catalog(language.Japanese, "File downloaded,% s,% d bytes", "ファイルをダウンロードしました: %s, %d バイト")
	catalog(language.Japanese, "Uploading object ...", "オブジェクトをアップロード中...")
	catalog(language.Japanese, "Uploaded file,% s", "ファイルをアップロードしました: %s")
	catalog(language.Japanese, "\"Selection Value:\" {{ .Val | red | cyan }}", "選択値: %s")
	catalog(language.Japanese, "What are you doing?", "何をしますか？")
	catalog(language.Japanese, "Which file do you upload?", "どのファイルをアップロードしますか？")
	catalog(language.Japanese, "Which files do you want to delete?", "どのファイルを削除しますか？")
	catalog(language.Japanese, "Which file do you want to download?", "どのファイルをダウンロードしますか？")
	catalog(language.Japanese, "The file exists. Overwrite? File name:% s, [Yy] / [Nn]", "ファイルが存在します。上書きしますか？ファイル名: %s, [Yy] / [Nn]")
	catalog(language.Japanese, "Object list created:", "オブジェクトリストが作成されました: ")
	catalog(language.Japanese, "s3ry needs an interactive terminal for the TUI. Run `s3ry --help` to see the non-interactive subcommands.", "TUIには対話型ターミナルが必要です。非対話型サブコマンドについては、`s3ry --help`を実行してください。")
}
