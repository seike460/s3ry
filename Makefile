build:
	go build -o s3ry cmd/main.go
install:
	echo "WIP"
release:
	$(eval latest_tag := $(shell git describe --abbrev=0 --tags))
	goxc
	ghr $(latest_tag) dist/snapshot/
