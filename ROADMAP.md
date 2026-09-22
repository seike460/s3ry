# Roadmap

## Current state

- The project is being rebuilt around the interactive `cmd/s3ry` binary.
- The S3 backend runs on AWS SDK for Go v2 with a shared session,
  per-region client caches, and transfer-manager multipart concurrency.
- Listing paginates through every page; prefix crawling can run in
  parallel. Delete and overwrite operations ask for confirmation first.
- The legacy promptui UI, the AWS SDK v1 backend, and the unreachable
  internal package groups were removed.
- CI covers build, vet, race tests, lint, MinIO integration tests,
  govulncheck, and cross-compilation.

## Next release: v3.0.0

The next release is planned to include:

- Add hierarchical browsing beyond the flat object list.
- Add non-interactive `ls`, `cp`, `rm`, and `presign` subcommands.
- Rebuild release automation (GoReleaser or equivalent).

## Later

- Preview improvements
- Copy and move
- Object versions
- Bookmarks
- Sync

## Performance

Performance figures will not be published until a reproducible harness is
in the repository and its measurement conditions are recorded.
