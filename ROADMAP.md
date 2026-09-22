# Roadmap

## Current state

- The project is built around the interactive `cmd/s3ry` binary on AWS SDK
  for Go v2, with a shared session, per-region client caches, and
  transfer-manager multipart concurrency.
- The object browser is hierarchical: common prefixes open as folders,
  `Esc` climbs, and long listings page through `Load more...` rows.
- Non-interactive subcommands (`ls`, `cat`, `rm`, `presign`) cover pipelines
  and non-TTY environments; all share the global AWS flags.
- `/` filtering, `HeadObject` metadata preview, in-TUI presigned URLs, and
  dry-run-counted folder deletes are available.
- S3-compatible endpoints work via `--endpoint`, `--path-style`, and
  `--no-sign-request`.
- Releases run through GoReleaser: GitHub assets, Linux packages, checksums,
  and the Homebrew tap formula.
- CI covers build, vet, race tests, lint, MinIO integration tests,
  govulncheck, cross-compilation, and Scrutinizer analysis.

## Shipped

- v3.0.0: the v2 rebuild — Bubble Tea UI, AWS SDK for Go v2, transfer
  manager concurrency, CI and release automation.
- v3.1.0: hierarchical browsing, `/` filtering, metadata preview, presigned
  URLs, folder deletes with dry-run counts, non-interactive subcommands, and
  S3-compatible endpoint flags.

## Later

- Copy and move (requires a `CopyObject` backend)
- Object versions
- Bookmarks
- Sync
- Bucket summary / storage posture views (size totals, incomplete
  multipart uploads, storage-class breakdown)
- Operation history and local-time timestamps

## Performance

Performance figures will not be published until a reproducible harness is
in the repository and its measurement conditions are recorded.
