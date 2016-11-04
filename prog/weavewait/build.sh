#!/bin/bash

go build -ldflags "-X github.com/weaveworks/weave/net.VethName=eth0 -linkmode external -extldflags -static -s" -tags iface -o r

