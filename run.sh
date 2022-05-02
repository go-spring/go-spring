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
            go test -race -count=1 ./... ;;
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

# 启动项目的 doc 页面
function doc() {

  project=$2
  array=(${project//-/ })
  dir=${array[0]}
  if [[ $dir != "spring" && $dir != "starter" ]]; then
    exit
  fi

  WORKSPACE=$1"/"$dir"/"$project
  echo $WORKSPACE
  echo ";; 首次启动加载时间较长，请耐心等待并手动刷新 doc 页面"

  MODULE_NAME=$project
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
}

# 命令包括: test、lint、doc.
case $1 in
  "doc")
    doc $(pwd) $2;;
  *)
    run $(pwd) $1 ;;
esac