#!/bin/bash

go build -ldflags "-X github.com/weaveworks/weave/net.VethName=eth0 -linkmode external -extldflags -s" -tags iface -o r

