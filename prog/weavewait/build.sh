#!/bin/bash

GOARCH=amd64 go build -ldflags "-X github.com/weaveworks/weave/net.VethName=eth0 -linkmode external -extldflags -static -s" -tags iface -o r-linux-amd64
GOARCH=ppc64le go build -ldflags "-X github.com/weaveworks/weave/net.VethName=eth0 -linkmode external -extldflags -static -s" -tags iface -o r-linux-ppc64le
