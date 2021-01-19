#!/bin/zsh

GOARCH=amd64 GOOS=linux  go build  -ldflags '-w -s' -o tcp-tunnle-server
