[![CI](https://github.com/seike460/s3ry/actions/workflows/ci.yml/badge.svg)](https://github.com/seike460/s3ry/actions/workflows/ci.yml)

# s3ry

s3ry は Amazon S3 用の対話型ターミナルクライアントです。Go で書かれており、Bubble Tea を使っています。

[English](README.md)

## ステータス

このプロジェクトは AWS SDK for Go v2 上で再構築中です。v2.0.0 のリリースノートは 2026-09-03 に修正済みです。

## 現在使える機能

- S3 バケットの一覧表示。
- バケットの階層ブラウズ：フォルダ（common prefix）は `Enter` で開き、`Esc` で戻ります。長い一覧は `Load more...` 行でページ単位に読み込みます。
- `/` で任意のリストをその場でフィルタ（タイトルと説明文への大文字小文字を区別しない部分一致）。
- オブジェクトの全メタデータをプレビュー（`p`）：`HeadObject` から Content-Type、ストレージクラス、バージョン ID、ユーザーメタデータを表示します。
- 選択中オブジェクトの署名付き GET URL をコピー（`P`）。有効期限は 1 時間 / 24 時間 / 7 日間から選択します。
- 選択したオブジェクトをカレントディレクトリへダウンロード。ダウンロードは AWS transfer manager による並列マルチパート転送です。
- カレントディレクトリ配下から、選択した非隠しファイルをバケットへアップロード（隠しファイル・隠しディレクトリはスキップ）。
- 選択したオブジェクトの削除、またはフォルダごとの削除 — 確認プロンプトには dry-run で取得した対象件数を先に表示します。削除モードでフォルダに `Enter` を押すと配下の全オブジェクトを削除します。フォルダ内に入って個別のオブジェクトを削除したい場合は `l` または `→` を押してください。ローカルファイルの上書き時にも確認を求めます。
- 選択したバケットの全オブジェクト一覧をカレントディレクトリの `ObjectList-<timestamp>.txt` へエクスポート。
- バケットや prefix を指定して直接起動：`s3ry s3://bucket` はバケットの操作メニューを開き、`s3ry s3://bucket/prefix`（パス指定、または末尾スラッシュ付きの `s3://bucket/`）はその位置のオブジェクトブラウザを直接開きます。
- 転送中に `Esc` を押すとキャンセルします。
- `--endpoint`、`--path-style`、`--no-sign-request` で MinIO や LocalStack などの S3 互換エンドポイントに対応。

### 非対話サブコマンド

```sh
s3ry ls [s3://bucket[/prefix]] [-o json]     # バケットまたはオブジェクトを一覧表示
s3ry cat s3://bucket/key                     # オブジェクトの内容を stdout へ出力
s3ry rm s3://bucket/key [--dry-run]          # オブジェクトを削除
s3ry rm s3://bucket/prefix/ [--dry-run]      # prefix 配下の全オブジェクトを削除
s3ry presign s3://bucket/key [--expires 2h]  # 署名付き GET URL を出力
```

すべてのサブコマンドはグローバルな AWS フラグ（`--profile`、`--region`、`--endpoint`、`--path-style`、`--no-sign-request`）を受け付けます。

![s3ry logo](doc/S3ry.png)

## 既知の制限

- 対話型 TUI は TTY が必要です。TTY がない環境では `ls`、`cat`、`rm`、`presign` の各サブコマンドを使ってください。

## インストール

### GitHub Releases

[GitHub Releases](https://github.com/seike460/s3ry/releases) からプラットフォーム向けのアセットをダウンロードしてください：

- `s3ry_Linux_x86_64.tar.gz`
- `s3ry_Linux_arm64.tar.gz`
- `s3ry_Darwin_arm64.tar.gz`
- `s3ry_Darwin_x86_64.tar.gz`
- `s3ry_Windows_x86_64.zip`
- `s3ry_Freebsd_x86_64.tar.gz`

### Homebrew

```sh
brew install seike460/tap/s3ry
```

### ソースからビルド

Go 1.27 以降が必要です：

```sh
make build   # dist/s3ry に出力
```

## 使い方

対話型クライアントを起動します：

```sh
s3ry
```

バイナリの `--help` 出力は次の通りです：

```text
interactive terminal client for Amazon S3

Usage:
  s3ry [s3://bucket/prefix] [flags]
  s3ry [command]

Available Commands:
  cat         Print an object's contents to stdout
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  ls          List buckets or objects without the TUI
  presign     Print a presigned GET URL for an object
  rm          Delete an object or a prefix without the TUI
  version     Show version information

Flags:
      --config string      Path to config file
      --endpoint string    S3 endpoint URL for S3-compatible services
  -h, --help               help for s3ry
      --lang string        Language (en, ja)
      --log-level string   Log level (debug, info, warn, error)
      --no-sign-request    Send unsigned requests for public endpoints
      --path-style         Force path-style S3 addressing (MinIO, LocalStack)
      --profile string     AWS profile to use
      --region string      AWS region to use
  -v, --verbose            Enable verbose logging
      --version            version for s3ry

Use "s3ry [command] --help" for more information about a command.
```

`Usage` 行と使用例の実行ファイル名は、起動時に使った名前に追従します。

`s3ry version` と `s3ry completion <shell>` のサブコマンドも利用できます。

### キーボード操作

- リストでは `↑`/`↓` または `k`/`j` で項目間を移動します。`PgUp`/`PgDn`、`Ctrl+B`/`Ctrl+F`、`Home`/`End` でページ・位置ジャンプができます。`Enter` または `Space` で現在の項目を選択します。
- 操作ビューでは `d` でダウンロード、`u` でアップロード、`Delete` で削除を開始します。
- オブジェクトビューでは `/` でリストをフィルタ、`p` でオブジェクトメタデータのプレビューを切り替え、`P` で署名付き URL を発行、`Enter` でフォルダを開き、`r` でリストを再読み込み、`?` でヘルプ、`s` で設定を開きます。`Esc` は親 prefix へ戻るか、操作ビューへ戻ります。
- バケットビューでは `r` でバケットの読み込みを再試行、`?` でヘルプ、`s` で設定を開きます。
- アップロードビューでは `r` でローカルファイルを再スキャン、`Esc` で操作ビューへ戻ります。
- 全体共通で `Ctrl+H` または `F1` でヘルプ、`Ctrl+S` で設定を開きます。
- `q` または `Ctrl+C` で終了します。

## 設定

### 環境変数

設定ローダーは次の変数を読み込みます：

- `AWS_REGION`
- `AWS_DEFAULT_REGION`（`aws.region` と `AWS_REGION` が未設定の場合）
- `AWS_PROFILE`
- `AWS_ENDPOINT_URL`（LocalStack などのカスタムエンドポイントは自動的にパススタイルアドレッシングになります）
- `S3RY_LANGUAGE`
- `S3RY_LOG_LEVEL`

これらはすべて起動時に S3 セッションへ適用されます。

### 設定ファイル

`--config` 指定がない場合、ローダーは次の相対パスを順に確認し、その後ホームディレクトリ配下のパスを確認します：

- `s3ry.yml`
- `s3ry.yaml`
- `.s3ry.yml`
- `.s3ry.yaml`
- `~/.s3ry.yml`
- `~/.s3ry.yaml`
- `~/.config/s3ry/config.yml`
- `~/.config/s3ry/config.yaml`

`--config PATH` を指定した場合はその YAML ファイルを読み込み、その後に環境変数を適用します。

設定型が定義する YAML キーは次の通りです：

- `aws.region`、`aws.profile`、`aws.endpoint`、`aws.path_style`、`aws.no_sign_request`
- `ui.language`、`ui.theme`
- `performance.concurrency`、`performance.part_size`、`performance.timeout`
- `logging.level`、`logging.format`、`logging.file`

`performance.concurrency` はマルチパート転送と prefix 一覧取得で使う並列 S3 ワーカー数を設定します。`performance.part_size` はマルチパートのチャンクサイズ（バイト単位、最小 5 MiB）です。`performance.timeout` は TUI からの各ブロッキング S3 リクエストの上限秒数です。

## 終了コード

| コード | 意味 |
| ---- | ------- |
| 0    | 成功 |
| 1    | 一般エラー |
| 2    | 使用法エラー（不明なフラグやコマンド、不正な引数） |
| 3    | バケットまたはオブジェクトが見つからない |
| 4    | アクセス拒否または認証情報なし |
| 130  | キャンセル（`Ctrl+C`、転送中の `Esc`、または SIGINT） |

## 開発

Go ツールチェーンと開発ツールは `.mise.toml` にピン留めされています（`mise install` で導入されます）。標準のチェックは次の通りです：

```sh
go build ./...
go test -race ./...
go vet ./...
make lint       # mise でピン留めされた golangci-lint 経由
make build      # dist/s3ry へスタンプ付きバイナリを出力
```

統合テストはローカルの MinIO コンテナに対して実行します。セットアップは
[docs/testing.md](docs/testing.md) を参照してください。`make bench` は
転送ベンチマークを記録し、リグレッション比較に使います。

## 設計ノート

- 起動時に 1 つの `s3.Session`（AWS SDK for Go v2）を作成し、すべてのビューで共有します。バケットのリージョンごとに S3 クライアントと transfer manager をキャッシュするため、クロスリージョンのバケットでも再起動は不要です。
- アップロードとダウンロードは AWS transfer manager を経由し、`performance.concurrency` 個の goroutine でマルチパート処理を並列化します。
- `Walk` は `Concurrency` 1 では辞書順に一覧取得し、それ以上では delimiter の prefix を並列に巡回します。オブジェクトブラウザは階層ページングに `ListPage`（delimiter + 継続トークン）を使います。
- 転送の進捗コールバックはワーカー goroutine 上で動作します。有界なブローカーチャネルが Bubble Tea の更新ループへ渡すため、描画が遅くても転送ワーカーをブロックせず、進捗報告はレースフリーです。
- ダウンロードはまず同じディレクトリの一時ファイルへ書き込み、成功時のみ公開します。上書きモード（`fail`、`skip`、`always`）に従い、`Esc` でキャンセルすると中途半端なファイルを削除します。
- S3 キーは検証され、`LocalPath` でローカルパスへマップされます。絶対パス、`..` トラバーサル、prefix エスケープは拒否します。
- UI は Bubble Tea のモデルツリーです。`cli` がフラグを解析し、`app` がルートモデルとグローバルキーを持ち、`views` が責務ごとの画面を、`components` が再利用可能なウィジェットを保持します。

### 拡張ポイント

- **操作**：`internal/ui/views/operation.go` の `operations` テーブルに 1 エントリ追加するだけです（キー、ラベル、遷移先ビュー）。switch 文を編集する必要はありません。
- **言語**：`internal/i18n/messages.go` にカタログマップを 1 つ追加するだけです。すべてのビューは注入された `Printer` 経由で描画します。
- **ビュー**：`tea.Model` を実装し、操作テーブルから構築して、`views.Deps` 経由で依存を配線します。遷移と転送の後始末はアプリ側が汎用的に処理します。
- **テーマ**：色は `internal/ui/components/theme.go` の名前付き定数で、すべてのウィジェットとビューで共有されます。

## ロードマップ

[ROADMAP.md](ROADMAP.md) を参照してください。

## コントリビューション

[CONTRIBUTING.md](CONTRIBUTING.md) を参照してください。

## ライセンス

このプロジェクトは MIT License でライセンスされています。[LICENSE](LICENSE) を参照してください。
