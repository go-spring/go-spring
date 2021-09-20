/*
 * Copyright 2012-2019 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-spring/go-spring/tools/gs/internal"
)

func init() {
	log.SetFlags(log.Lshortfile)
}

const help = `(v0.0.1) command:
  gs pull spring-*/starter-* [branch]
  gs push spring-*/starter-*
  gs remove spring-*/starter-*
  gs release tag`

var commands = map[string]func(rootDir string){
	"pull":    pull,
	"push":    push,
	"remove":  remove,
	"release": release,
}

// arg 获取命令行参数
func arg(index int) string {
	if len(os.Args) > index {
		return os.Args[index]
	}
	panic("not enough arg")
}

// projectsXml 配置文件
var projectsXml internal.ProjectsXml

func main() {

	fmt.Println(help)
	defer func() { fmt.Println() }()

	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
			os.Exit(-1)
		}
	}()

	cmd := arg(1)
	fn, ok := commands[cmd]
	if !ok {
		panic("error command " + cmd)
	}

	// 获取工作目录
	rootDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	// 加载 projects.xml 配置文件
	projectFile := path.Join(rootDir, "projects.xml")
	err = projectsXml.Read(projectFile)
	if err != nil {
		panic(errors.New("read projects.xml error"))
	}

	count := len(projectsXml.Projects)
	defer func() {
		if count != len(projectsXml.Projects) {
			err = projectsXml.Save(projectFile)
			if err != nil {
				panic(err)
			}
		}
	}()

	fmt.Print(os.Args, " 输入 Yes 执行该命令: ")
	input, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	if strings.TrimSpace(input) != "Yes" {
		os.Exit(-1)
	}

	zip(rootDir)
	fn(rootDir)
}

// zip 备份本地文件
func zip(rootDir string) {
	backupDir := path.Dir(rootDir)
	baseName := path.Base(rootDir)
	now := time.Now().Format("20060102150405")
	zipFile := fmt.Sprintf("%s-%s.zip", baseName, now)
	cmd := exec.Command("zip", "-qr", "-x=*/vendor/*", zipFile, "./"+baseName)
	cmd.Dir = backupDir
	if err := cmd.Run(); err != nil {
		panic(err)
	}
}

// pull 拉取远程项目
func pull(rootDir string) {

	project := arg(2)
	branch := arg(3)

	prefix, projectDir, err := internal.GetProjectPath(rootDir, project)
	if os.IsNotExist(err) {
		err = os.MkdirAll(projectDir, os.ModePerm)
		if err != nil {
			panic(err)
		}
	} else if err != nil {
		panic(err)
	}

	projectsXml.Add(internal.Project{
		Name:   project,
		Branch: branch,
		Dir:    fmt.Sprintf("%s/%s", prefix, project),
		Url:    fmt.Sprintf("https://github.com/go-spring/%s.git", project),
	})
}

// push 推送更新到远程项目
func push(rootDir string) {
	project := arg(2)
	internal.Push(project, rootDir)
}

// remove 移除远程项目
func remove(rootDir string) {

	project := arg(2)
	_, projectDir, err := internal.GetProjectPath(rootDir, project)
	if err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}
	}

	if err = os.RemoveAll(projectDir); err != nil {
		panic(err)
	}
	projectsXml.Remove(project)
}

// release 发布远程项目
func release(rootDir string) {

	tag := arg(2)
	err := filepath.Walk(rootDir, func(walkFile string, _ os.FileInfo, err error) error {

		if err != nil {
			return err
		}

		if path.Base(walkFile) != "go.mod" {
			return nil
		}

		//fmt.Println(walkFile)
		fileData, err := ioutil.ReadFile(walkFile)
		if err != nil {
			return nil
		}

		outBuf := bytes.NewBuffer(nil)
		r := bufio.NewReader(strings.NewReader(string(fileData)))
		for {
			line, isPrefix, err := r.ReadLine()
			if len(line) > 0 && err != nil {
				panic(err)
			}
			if isPrefix {
				panic(errors.New("ReadLine returned prefix"))
			}
			if err != nil {
				if err != io.EOF {
					panic(err)
				}
				break
			}
			s := strings.TrimSpace(string(line))
			if strings.HasPrefix(s, "github.com/go-spring/spring-") ||
				strings.HasPrefix(s, "github.com/go-spring/starter-") {
				index := strings.LastIndexByte(s, ' ')
				if index <= 0 {
					panic(errors.New(s))
				}
				b := append(line[:index+2], []byte(tag)...)
				outBuf.Write(b)
			} else {
				outBuf.Write(line)
			}
			outBuf.WriteString("\n")
		}

		//fmt.Println(outBuf.String())
		return ioutil.WriteFile(walkFile, outBuf.Bytes(), os.ModePerm)
	})

	if err != nil {
		panic(err)
	}

	// 提交代码更新
	{
		cmd := exec.Command("bash", "-c", fmt.Sprintf("git commit -am \"publish %s\"", tag))
		cmd.Dir = rootDir
		b, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Errorf("err %v with output %s", err, b))
		}
	}

	// 遍历所有项目，推送远程更新
	for _, project := range projectsXml.Projects {
		internal.Push(project.Name, rootDir)
	}

	// 创建临时目录
	now := time.Now().Format("20060102150405")
	buildDir := path.Join(rootDir, "..", "go-spring-build-"+now)
	err = os.MkdirAll(buildDir, os.ModePerm)
	if err != nil {
		panic(err)
	}

	// 遍历所有项目，推送远程标签
	for _, project := range projectsXml.Projects {
		internal.Clone(buildDir, project.Name, project.Url)
		projectDir := filepath.Join(buildDir, project.Name)
		internal.Release(projectDir, project.Name, tag)
	}
}
