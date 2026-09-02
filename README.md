[![CI](https://github.com/seike460/s3ry/actions/workflows/ci.yml/badge.svg)](https://github.com/seike460/s3ry/actions/workflows/ci.yml)

# s3ry

s3ry is an interactive terminal client for Amazon S3. It is written in Go and uses Bubble Tea.

## Status

This project is being rebuilt. The current binary works but is limited; see Known limitations. The v2.0.0 release notes were corrected on 2026-09-03.

## What works today

- List S3 buckets.
- Browse objects in a selected bucket as a flat list. The current UI displays the first 1000 keys.
- Download a selected object to the current directory.
- Upload a selected file that is not hidden (hidden files and directories are skipped) from the current directory tree to the selected bucket.
- Delete a selected object.
- Export the selected bucket's object list to `ObjectList-<timestamp>.txt` in the current directory; the 1000-object limit also applies to export.

![s3ry logo](doc/S3ry.png)

## Known limitations

- Object browsing uses one `ListObjectsV2` request and is limited to the first 1000 keys. Pagination is not connected to the UI.
- Delete and overwrite operations do not ask for confirmation.
- `--region` and `AWS_REGION` affect only the bucket list; object operations (download / upload / delete / export) always use a client configured for `ap-northeast-1`.
- `--profile` and `AWS_ENDPOINT_URL` are read or accepted but are not applied to the active S3 client.
- `--lang` and `S3RY_LANGUAGE` have no effect in the current UI.
- `--log-level` and `--verbose` do not change the logging output level; the UI does not emit logs.
- The binary exits with an error when stdin or stdout is not a TTY. Non-interactive commands are not available.
- The Japanese UI is temporarily unavailable while it is being rebuilt.

## Installation

### GitHub Releases

Download the asset for your platform from [GitHub Releases](https://github.com/seike460/s3ry/releases):

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

### Build from source

Use Go 1.25 or later:

```sh
go build ./cmd/s3ry
```

## Usage

Start the interactive client:

```sh
s3ry
```

The current binary's `--help` output is:

```text
s3ry - interactive terminal client for Amazon S3

Usage: s3ry [OPTIONS]

Options:
  -config string
    	Path to config file
  -h	Show help (short)
  -help
    	Show help
  -lang string
    	Language (en, ja)
  -log-level string
    	Log level (debug, info, warn, error)
  -profile string
    	AWS profile to use
  -region string
    	AWS region to use
  -v	Enable verbose logging (short)
  -verbose
    	Enable verbose logging
  -version
    	Show version information

Examples:
  s3ry                      # Start with the Bubble Tea UI
  s3ry --region us-west-2   # Use specific AWS region
  s3ry --profile dev      # Use specific AWS profile
  s3ry --config ./s3ry.yml # Use a specific configuration file
  s3ry --lang en          # Use English language

Environment Variables:
  AWS_REGION            # AWS region
  AWS_PROFILE           # AWS profile
  S3RY_LANGUAGE         # Language (en, ja)
  S3RY_LOG_LEVEL        # Log level
```

The executable name in the `Usage` and example lines follows the name used to start the binary.

### Keyboard controls

- In lists, `↑`/`↓` or `k`/`j` move between items. `PgUp`/`PgDn`, `Ctrl+B`/`Ctrl+F`, and `Home`/`End` provide page and position navigation. `Enter` or `Space` selects the current item.
- In the operation view, `d` starts download, `u` starts upload, and `Delete` starts delete.
- In the object view, `p` toggles the object metadata preview, `r` reloads the list, `?` opens help, `s` opens settings, and `Esc` returns to the operation view.
- In the bucket view, `r` retries loading buckets, `?` opens help, and `s` opens settings.
- In the upload view, `r` rescans local files and `Esc` returns to the operation view.
- Globally, `Ctrl+H` or `F1` opens help, and `Ctrl+S` opens settings.
- `q` or `Ctrl+C` exits the client.

## Configuration

### Environment variables

The configuration loader reads these variables:

- `AWS_REGION`
- `AWS_DEFAULT_REGION` when the configured region is still the default
- `AWS_PROFILE`
- `AWS_ENDPOINT_URL`
- `S3RY_LANGUAGE`
- `S3RY_LOG_LEVEL`

The `AWS_PROFILE` and `AWS_ENDPOINT_URL` values are currently not applied to the active S3 client; see Known limitations.

### Configuration files

Without `--config`, the loader checks these relative paths and then the home-directory paths:

- `s3ry.yml`
- `s3ry.yaml`
- `.s3ry.yml`
- `.s3ry.yaml`
- `~/.s3ry.yml`
- `~/.s3ry.yaml`
- `~/.config/s3ry/config.yml`
- `~/.config/s3ry/config.yaml`

With `--config PATH`, the specified YAML file is read and environment variables are applied afterward.

The YAML keys defined by the configuration type are:

- `aws.region`, `aws.profile`, `aws.endpoint`
- `ui.language`, `ui.theme`
- `performance.workers`, `performance.chunk_size`, `performance.timeout`, `performance.max_concurrent_downloads`, `performance.max_concurrent_uploads`
- `logging.level`, `logging.format`, `logging.file`
- `log_level`, `log_format`, `log_file`, `debug_level`, `debug_file`, `profile_dir`, `environment`, `version`

In the current binary, only `aws.region` affects behavior, and only for the bucket list; the other keys are read but not used. `logging.level` does not change output because the UI does not emit logs.

## Roadmap

See [ROADMAP.md](ROADMAP.md).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE).
