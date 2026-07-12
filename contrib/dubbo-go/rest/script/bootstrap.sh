#! /usr/bin/env bash
CURDIR=$(cd $(dirname $0); pwd)

if [ "X$1" != "X" ]; then
    RUNTIME_ROOT=$1
else
    RUNTIME_ROOT=${CURDIR}
fi

export APP_RUNTIME_ROOT=$RUNTIME_ROOT

exec "$CURDIR/bin/dubbo-go-rest"
