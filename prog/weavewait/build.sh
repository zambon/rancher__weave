#!/bin/bash

OS="$(uname -s)"
ARCH="$(uname -m)"

go build -ldflags "-X github.com/weaveworks/weave/net.VethName=eth0 -linkmode external -extldflags -static -s" -tags iface -o "r-$OS-$ARCH"
