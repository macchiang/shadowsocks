#!/bin/zsh

GOARCH=amd64 GOOS=linux  go build  -ldflags '-w -s' -o tcp-tunnle-client

scp tcp-tunnle-client root@192.168.31.103:~