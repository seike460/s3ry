package i18n

import (
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func init() {
	// English messages
	message.SetString(language.English, "Which bucket do you use?", "Which bucket do you use?")
	message.SetString(language.English, "download", "download")
	message.SetString(language.English, "upload", "upload")
	message.SetString(language.English, "delete object", "delete object")
	message.SetString(language.English, "create object list", "create object list")
	message.SetString(language.English, "Searching for buckets ...", "Searching for buckets ...")
	message.SetString(language.English, "Searching for objects ...", "Searching for objects ...")
	message.SetString(language.English, "Number of objects: ", "Number of objects: ")
	message.SetString(language.English, "Downloading object ...", "Downloading object ...")
	message.SetString(language.English, "File downloaded,% s,% d bytes", "File downloaded, %s, %d bytes")
	message.SetString(language.English, "Uploading object ...", "Uploading object ...")
	message.SetString(language.English, "Uploaded file,% s", "Uploaded file, %s")
	message.SetString(language.English, "\"Selection Value:\" {{ .Val | red | cyan }}", "Selection Value: %s")
	message.SetString(language.English, "What are you doing?", "What are you doing?")
	message.SetString(language.English, "Which file do you upload?", "Which file do you upload?")
	message.SetString(language.English, "Which files do you want to delete?", "Which files do you want to delete?")
	message.SetString(language.English, "Which file do you want to download?", "Which file do you want to download?")
	message.SetString(language.English, "The file exists. Overwrite? File name:% s, [Yy] / [Nn]", "The file exists. Overwrite? File name: %s, [Yy] / [Nn]")
	message.SetString(language.English, "Object list created:", "Object list created: ")

	// Japanese messages
	message.SetString(language.Japanese, "Which bucket do you use?", "どのバケットを使用しますか？")
	message.SetString(language.Japanese, "download", "ダウンロード")
	message.SetString(language.Japanese, "upload", "アップロード")
	message.SetString(language.Japanese, "delete object", "オブジェクトを削除")
	message.SetString(language.Japanese, "create object list", "オブジェクトリストを作成")
	message.SetString(language.Japanese, "Searching for buckets ...", "バケットを検索中...")
	message.SetString(language.Japanese, "Searching for objects ...", "オブジェクトを検索中...")
	message.SetString(language.Japanese, "Number of objects: ", "オブジェクト数: ")
	message.SetString(language.Japanese, "Downloading object ...", "オブジェクトをダウンロード中...")
	message.SetString(language.Japanese, "File downloaded,% s,% d bytes", "ファイルをダウンロードしました: %s, %d バイト")
	message.SetString(language.Japanese, "Uploading object ...", "オブジェクトをアップロード中...")
	message.SetString(language.Japanese, "Uploaded file,% s", "ファイルをアップロードしました: %s")
	message.SetString(language.Japanese, "\"Selection Value:\" {{ .Val | red | cyan }}", "選択値: %s")
	message.SetString(language.Japanese, "What are you doing?", "何をしますか？")
	message.SetString(language.Japanese, "Which file do you upload?", "どのファイルをアップロードしますか？")
	message.SetString(language.Japanese, "Which files do you want to delete?", "どのファイルを削除しますか？")
	message.SetString(language.Japanese, "Which file do you want to download?", "どのファイルをダウンロードしますか？")
	message.SetString(language.Japanese, "The file exists. Overwrite? File name:% s, [Yy] / [Nn]", "ファイルが存在します。上書きしますか？ファイル名: %s, [Yy] / [Nn]")
	message.SetString(language.Japanese, "Object list created:", "オブジェクトリストが作成されました: ")
}