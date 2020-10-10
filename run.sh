#!/bin/bash

# 当前目录是否是 Go 项目目录
function isProjectDir(){
  if [ -f $1"/go.mod" ]; then
    return $((1))
  fi
  return $((0))
}

# 找到 Go 项目目录并执行命令
function run(){
  for element in `ls $1`
  do
    dir_or_file=$1"/"$element
    if [ -d $dir_or_file ]; then
      isProjectDir $dir_or_file
      if [ $? -eq 0 ]; then
        run $dir_or_file $2
        cd $dir_or_file
      else
        echo $dir_or_file
        cd $dir_or_file
        case $2 in
          "test")
            # 执行当前目录及子目录下的测试用例
            go test -count=1 ./... ;;
          "lint")
            # https://github.com/golangci/golangci-lint
            golangci-lint run ;;
          *)
            ;;
        esac
      fi
    fi
  done
}

# 命令包括: test、lint.
run $(pwd) $1