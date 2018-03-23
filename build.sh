#!/bin/sh

echo "start build s3ry"

name=s3ry

GOOS=linux GOARCH=amd64 go build -o ./bin/linux64/$name
GOOS=linux GOARCH=386 go build -o ./bin/linux386/$name

GOOS=windows GOARCH=386 go build -o ./bin/windows386/$name.exe
GOOS=windows GOARCH=amd64 go build -o ./bin/windows64/$name.exe

GOOS=darwin GOARCH=386 go build -o ./bin/darwin386/$name
GOOS=darwin GOARCH=amd64 go build -o ./bin/darwin64/$name

echo "end build s3ry"
