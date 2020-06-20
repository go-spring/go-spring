#!/bin/bash

# 执行当前目录及子目录下的测试用例
go test -cover -coverprofile=covprofile -count=1 ./...
go tool cover -html=covprofile -o coverage.html
