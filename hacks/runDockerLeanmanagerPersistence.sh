#!/usr/bin/env bash

# Only for Fedora, selinux issues, execute: chcon -Rt svirt_sandbox_file_t .
docker run --rm -e LEANMANAGER_TOKEN=$LEANMANAGER_TOKEN -e LEANMANAGER_PATHDB=/mnt -v $(pwd):/mnt antonmry/leanmanager:latest
