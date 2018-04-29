#!/bin/sh
echo "start build s3ry"
set -e
latest_tag=$(git describe --abbrev=0 --tags)
goxc
ghr $latest_tag dist/snapshot/
echo "end build s3ry"
