#!/usr/bin/env bash
RUN_NAME="dubbo-go-rest"

mkdir -p output/bin
cp script/* output/
chmod +x output/bootstrap.sh

# The provider is the deployable service; the consumer is a discovery client
# exercised by check.sh, so only the provider is packaged here.
if [ "$IS_SYSTEM_TEST_ENV" != "1" ]; then
    go build -o output/bin/${RUN_NAME} ./provider
else
    go test -c -covermode=set -o output/bin/${RUN_NAME} -coverpkg=./... ./provider
fi
