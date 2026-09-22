# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Non-interactive subcommands: `ls` (buckets or objects, `-o json`), `cat`, `rm` (object or prefix, `--dry-run`), and `presign` (`--expires`).
- `--endpoint`, `--path-style`, and `--no-sign-request` flags plus `aws.endpoint`, `aws.path_style`, and `aws.no_sign_request` config keys for S3-compatible services.
- An optional `s3://bucket[/prefix]` positional argument that starts the TUI directly at a bucket or prefix.
- Hierarchical object browsing: common prefixes appear as folder rows, `Enter` descends, `Esc` ascends, and truncated listings offer a `Load more...` row.
- `/` incremental filtering on every list.
- Object preview (`p`) now shows full `HeadObject` metadata: Content-Type, storage class, version ID, and user metadata.
- Presigned URL generation in the TUI (`P`) with a 1h/24h/7d expiry chooser and OSC52 clipboard copy.
- Folder delete removes every object under the prefix via `DeletePrefix`; the confirmation prompt shows the dry-run object count.

### Changed

- The AWS connection flags moved to persistent flags so the subcommands share them.
- The object list shows keys relative to the current prefix and reports the prefix in the context line.

## [3.0.0] - 2026-09-22

### Added

- A CI workflow covering build, vet, race tests, golangci-lint, integration tests against MinIO, vulnerability checks with govulncheck, and cross-compilation.
- CHANGELOG.md, CONTRIBUTING.md, SECURITY.md, issue and pull request templates, and Dependabot configuration.
- `version` and `completion` subcommands; typed exit codes (1 general, 2 usage, 3 not found, 4 access denied or no credentials, 130 canceled).
- Confirmation prompts before object deletion and local file overwrite.
- `make build` stamps version, commit, and build date into the binary via `-ldflags` and builds with `-trimpath`.
- `internal/s3` on AWS SDK for Go v2: a shared session with per-region client and transfer-manager caches, multipart upload/download with progress callbacks, recursive `DownloadPrefix`/`UploadDir`, batch delete with single-delete fallback, presigned GET, and concurrent prefix walking.

### Removed

- The GitHub Actions release workflow; releases run through GoReleaser locally.
- The `cmd/s3ry-tui` command.
- The legacy promptui UI and the public Go package `github.com/seike460/s3ry`.
- The AWS SDK for Go v1 backend (`internal/legacy`), the worker package, and the obsolete `pkg/interfaces` and `pkg/types` packages.
- The former flags `--legacy-ui`, `--new-ui`, `--bubbles`, and `--modern-backend`.
- The `ui.mode` and `S3RY_UI_MODE` settings.
- The desktop, web, and vscode commands and extensions.
- The terraform provider, SDK, Helm chart, and the Scoop, AUR, Snap, Docker, and Chocolatey packaging files and GoReleaser settings.
- Unreachable internal package groups including `sla`, `chaos`, `security`, `ai`, `analytics`, `telemetry`, `backup`, `dashboard`, `api`, `docs`, `updater`, and `cloud`.
- The CodeGuru workflow, the codecov configuration files, and the Dockerfile.

### Changed

- A non-TTY invocation no longer starts the interactive UI and exits with an error.
- The CLI is built on cobra; `--help` output changed.
- The TUI runs on AWS SDK for Go v2; region, profile, endpoint, concurrency, and part-size settings are honored end to end.
- `--config` now reads the specified configuration file.
- Timeouts are reported as "timed out" and exit with code 1 instead of being reported as cancellations.
- The Go directive is 1.27.

### Fixed

- `go.sum` is tracked.
- `.gitignore` no longer hides new files under `cmd/s3ry`.
- List key input that was dropped within the 50ms window and the three failing tests were fixed.
- The upload file picker was always empty.
- Object listings paginate through every page instead of stopping at 1000 keys.
- Quitting or switching views during a transfer now cancels the worker and releases the progress broker instead of leaking goroutines.
- The progress bar no longer panics on very narrow terminals or when a transfer overshoots its announced total.

### Security

- Updated `gopkg.in/yaml.v2` to v2.4.0 for `GO-2022-0956`, `GO-2021-0061`, and `GO-2020-0036`.
- Updated `golang.org/x/text` to v0.39.0 and `golang.org/x/sys` to v0.44.0.

## [2.0.0] - 2025-06-18

Release notes were corrected on 2026-09-03; the unsupported claims were retracted.

## [0.2] - 2019-12-20

## [0.1.4] - 2019-12-20

## [0.1.3] - 2018-11-24

## [0.1.2] - 2018-11-24

## [0.1.1] - 2018-11-14

## [0.1] - 2018-04-29

[Unreleased]: https://github.com/seike460/s3ry/compare/v3.0.0...HEAD
[3.0.0]: https://github.com/seike460/s3ry/releases/tag/v3.0.0
[2.0.0]: https://github.com/seike460/s3ry/releases/tag/v2.0.0
[0.2]: https://github.com/seike460/s3ry/releases/tag/0.2
[0.1.4]: https://github.com/seike460/s3ry/releases/tag/0.1.4
[0.1.3]: https://github.com/seike460/s3ry/releases/tag/0.1.3
[0.1.2]: https://github.com/seike460/s3ry/releases/tag/0.1.2
[0.1.1]: https://github.com/seike460/s3ry/releases/tag/0.1.1
[0.1]: https://github.com/seike460/s3ry/releases/tag/0.1
