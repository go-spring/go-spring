#!/bin/bash

set -ex

export GOPATH
export GOROOT=/usr/local/go1.26.6
export PATH=${GOROOT}/bin:${GOPATH}/bin:${PATH}:${GOBIN}
export GOPROXY=direct
export GOSUMDB=off

echo "Building..."
if make; then
	echo "Build success!"
else
	echo "Build failed!"
	exit 1
fi
