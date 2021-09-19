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
	"errors"
	"fmt"
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

// getProjectPrefix 获取项目前缀
func getProjectPrefix(project string) string {
	prefix := strings.Split(project, "-")[0]
	if prefix != "spring" && prefix != "starter" {
		panic(errors.New("error project name"))
	}
	return prefix
}

func getProjectPath(rootDir, project string) (prefix, projectDir string, err error) {
	prefix = getProjectPrefix(project)
	projectDir = filepath.Join(rootDir, prefix, project)
	stat, err := os.Stat(projectDir)
	if os.IsNotExist(err) {
		return prefix, projectDir, err
	} else if err != nil {
		return "", "", err
	}
	if !stat.IsDir() {
		return "", "", fmt.Errorf("%s exist but not dir", projectDir)
	}
	return prefix, projectDir, nil
}

// pull 拉取远程项目
func pull(rootDir string) {

	project := arg(2)
	branch := arg(3)

	prefix, projectDir, err := getProjectPath(rootDir, project)
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
	prefix, projectDir, err := getProjectPath(rootDir, project)
	if err != nil {
		panic(err)
	}

	var commitID string
	{
		cmd := exec.Command("bash", "-c", fmt.Sprintf("git log --pretty=format:\"%%H\" %s/%s | awk \"NR==1\"", prefix, project))
		cmd.Dir = rootDir
		b, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Errorf("err %v with output %s", err, b))
		}
		commitID = string(b)
	}

	var commitMsg string
	{
		cmd := exec.Command("bash", "-c", fmt.Sprintf("git log --pretty=format:\"%%s\" %s/%s | awk \"NR==1\"", prefix, project))
		cmd.Dir = rootDir
		b, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Errorf("err %v with output %s", err, b))
		}
		commitMsg = string(b)
	}

	tempPath := filepath.Join(os.TempDir(), project)
	if err = os.RemoveAll(tempPath); err != nil {
		panic(err)
	}

	{
		repository := fmt.Sprintf("https://github.com/go-spring/%s.git", project)
		cmd := exec.Command("bash", "-c", fmt.Sprintf("git clone %s", repository))
		cmd.Dir = os.TempDir()
		b, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Errorf("err %v with output %s", err, b))
		}
		fmt.Printf("clone %s to temp dir success\n", project)
	}

	{
		cmd := exec.Command("bash", "-c", fmt.Sprintf("git log --pretty=format:\"%%B\" | awk \"NR==1\""))
		cmd.Dir = tempPath
		b, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Errorf("err %v with output %s", err, b))
		}

		// 远程仓库是最新版本
		if string(b) == commitID {
			return
		}
	}

	{
		cmd := exec.Command("bash", "-c", fmt.Sprintf("rm -rf %s/*", tempPath))
		cmd.Dir = tempPath
		b, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Errorf("err %v with output %s", err, b))
		}
	}

	{
		cmd := exec.Command("bash", "-c", fmt.Sprintf("cp -rf %s/* %s", projectDir, tempPath))
		b, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Errorf("err %v with output %s", err, b))
		}
	}

	{
		cmd := exec.Command("bash", "-c", "git add .")
		cmd.Dir = tempPath
		b, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Errorf("err %v with output %s", err, b))
		}
	}

	{
		cmd := exec.Command("bash", "-c", fmt.Sprintf("git commit -am \"%s%s\"", commitID, commitMsg))
		cmd.Dir = tempPath
		b, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Errorf("err %v with output %s", err, b))
		}
	}

	{
		cmd := exec.Command("bash", "-c", "git push -f")
		cmd.Dir = tempPath
		b, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Errorf("err %v with output %s", err, b))
		}
		fmt.Printf("push %s to remote dir success", project)
	}
}

// remove 移除远程项目
func remove(rootDir string) {

	project := arg(2)
	_, projectDir, err := getProjectPath(rootDir, project)
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

}
