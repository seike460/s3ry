# Roadmap

## Current state

- The project is being rebuilt around the interactive `cmd/s3ry` binary.
- The legacy promptui UI and the old public root Go package were removed.
- Unreachable internal packages and unused secondary application and packaging files were removed.

## Next release: v3.0.0

The next release is planned to include:

- Migrate the S3 backend to `aws-sdk-go-v2`.
- Add hierarchical browsing and pagination.
- Ask for confirmation before deletion and overwrite operations.
- Fix region and profile handling.
- Add non-interactive `ls`, `cp`, `rm`, and `presign` subcommands.
- Restore the Japanese UI.
- Add CI and release automation.

## Later

- Preview improvements
- Copy and move
- Object versions
- Bookmarks
- Sync

## Performance

Performance figures will not be published until a reproducible harness is in the repository and its measurement conditions are recorded.
