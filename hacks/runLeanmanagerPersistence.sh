#!/usr/bin/env bash

go run -race $GOPATH/src/github.com/antonmry/leanmanager/main.go -t $LEANMANAGER_TOKEN -d $GOPATH/src/github.com/antonmry/leanmanager
