build:
	$(eval latest_tag := $(shell git describe --abbrev=0 --tags))
	goxc
	ghr $(latest_tag) dist/snapshot/
