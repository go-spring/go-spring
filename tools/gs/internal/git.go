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

package internal

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// getProjectPrefix 获取项目前缀
func getProjectPrefix(project string) string {
	prefix := strings.Split(project, "-")[0]
	if prefix != "spring" && prefix != "starter" {
		panic(errors.New("error project name"))
	}
	return prefix
}

func GetProjectPath(rootDir, project string) (prefix, projectDir string, err error) {
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

// Push 推送更新到远程项目
func Push(project string, rootDir string) {

	prefix, projectDir, err := GetProjectPath(rootDir, project)
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
		cmd := exec.Command("bash", "-c", fmt.Sprintf("git status"))
		cmd.Dir = tempPath
		b, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Errorf("err %v with output %s", err, b))
		}
		if bytes.Contains(b, []byte("nothing to commit, working tree clean")) {
			return
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
		fmt.Printf("push %s to remote dir success\n", project)
	}
}

// Clone 克隆远程项目
func Clone(buildDir, project, repository string) {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("git clone %s", repository))
	cmd.Dir = buildDir
	b, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Errorf("err %v with output %s", err, b))
	}
	fmt.Printf("clone %s to temp dir success\n", project)
}

// Release 发布远程项目
func Release(projectDir, project, tag string) {

	{
		cmd := exec.Command("bash", "-c", "git tag "+tag)
		cmd.Dir = projectDir
		b, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Errorf("err %v with output %s", err, b))
		}
		fmt.Printf("push %s to remote dir success", project)
	}

	{
		cmd := exec.Command("bash", "-c", "git push -f")
		cmd.Dir = projectDir
		b, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Errorf("err %v with output %s", err, b))
		}
		fmt.Printf("push %s to remote dir success", project)
	}
}
