#!/bin/bash

set -e

workspace=$(cd "$(dirname "$0")" && pwd -P)
cd "$workspace"

app=GS_PROJECT_NAME
exec ./bin/"$app"
