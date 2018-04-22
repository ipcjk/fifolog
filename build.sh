#!/usr/bin/env bash

# X-compile everything ;-)
env GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o fifolog *.go
env GOOS=darwin GOARCH=amd64 go  build -ldflags="-s -w" -o fifolog.mac *.go

# pack our executables
upx --force fifolog*
