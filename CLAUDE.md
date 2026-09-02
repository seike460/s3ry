# s3ry

s3ry is an interactive terminal client for Amazon S3 written in Go with Bubble Tea.

## Build, test, and lint

```sh
go build ./...
go test -race ./...
go vet ./...
make lint
```

## Quality rules

- Do not add unreachable code.
- Do not write placeholder text such as "In a real implementation".
- Test behavior, not only compilation.
- Put only verified claims in README.md.
- Use Conventional Commits.

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidance.
