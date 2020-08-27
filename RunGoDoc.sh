#!/bin/bash

WORKSPACE=$(cd `dirname $0` && pwd -P)

# NOTE: 只需要修改这里
MODULE_NAME=starter-db

PACKAGE_PATH=github.com/go-spring
export GOPATH=/tmp/godoc-${MODULE_NAME}
MODULE_PATH=${GOPATH}/src/${PACKAGE_PATH}

rm -rf $MODULE_PATH/$MODULE_NAME &> /dev/null

mkdir -p $GOPATH/bin
mkdir -p $MODULE_PATH

ln -sf $WORKSPACE ${MODULE_PATH}/${MODULE_NAME}

cd $MODULE_PATH/$MODULE_NAME

python -m webbrowser "http://localhost:6060/pkg/github.com/go-spring/"${MODULE_NAME}
godoc