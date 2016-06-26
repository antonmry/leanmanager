#!/usr/bin/env bash

rm -f /tmp/leanmanager.db
go run -race $GOPATH/src/github.com/antonmry/leanmanager/main.go -t $LEANMANAGER_TOKEN
