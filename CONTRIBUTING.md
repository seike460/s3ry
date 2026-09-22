# Contributing

## Development environment

Use Go 1.27 or later. Work from a clone of the repository and keep changes focused on the current command and packages. [mise](https://mise.jdx.dev) users can run `mise install` to get the pinned Go, golangci-lint, and gofumpt versions.

## Build, test, and lint

```sh
go build ./...
go test -race ./...
go vet ./...
make lint
```

Run the relevant checks before opening a pull request. Add or update tests for behavior that changes.

## Pull requests

- Keep each change small and focused.
- Include tests for new or changed behavior.
- Use Conventional Commits for commit messages.
- Add only README claims that can be verified from the implementation or command output.
- Do not add placeholder code or text such as "In a real implementation".
- Do not add unreachable packages.

## Issues

Include a clear title, the s3ry version, operating system, Go version when relevant, reproduction steps, expected behavior, actual behavior, and sanitized logs. Do not include credentials or other secrets.
