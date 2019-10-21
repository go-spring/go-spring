#!/bin/bash

WORKSPACE=$(cd `dirname $0` && pwd -P)

MODULE_NAME=go-spring
PACKAGE_PATH=github.com/go-spring

export GOPATH=/tmp/godoc-${MODULE_NAME}
MODULE_PATH=${GOPATH}/src/${PACKAGE_PATH}

rm -rf $MODULE_PATH/$MODULE_NAME &> /dev/null

mkdir -p $GOPATH/bin
mkdir -p $MODULE_PATH

ln -sf $WORKSPACE ${MODULE_PATH}/${MODULE_NAME}

cd $MODULE_PATH/$MODULE_NAME

python -m webbrowser "http://localhost:6060/pkg/github.com/go-spring/go-spring/"
godoc