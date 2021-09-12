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
	"encoding/xml"
	"io/ioutil"
	"os"
	"sort"
)

type Project struct {
	Name   string `xml:"name"`
	Dir    string `xml:"dir"`
	Url    string `xml:"url"`
	Branch string `xml:"branch"`
}

type ProjectsXml struct {
	XMLName  xml.Name  `xml:"projects"`
	Projects []Project `xml:"project"`
}

// Add 添加一个项目
func (p *ProjectsXml) Add(project Project) {
	p.Projects = append(p.Projects, project)
}

// Find 查找一个项目，成功返回 true，失败返回 false
func (p *ProjectsXml) Find(project string) (Project, bool) {
	for _, t := range p.Projects {
		if t.Name == project {
			return t, true
		}
	}
	return Project{}, false
}

// Remove 移除一个项目
func (p *ProjectsXml) Remove(project string) {
	index := -1
	for i := 0; i < len(p.Projects); i++ {
		if p.Projects[i].Name == project {
			index = i
			break
		}
	}
	if index >= 0 {
		projects := append(p.Projects[:index], p.Projects[index+1:]...)
		p.Projects = projects
	}
}

// Read 读取配置文件
func (p *ProjectsXml) Read(filename string) error {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return xml.Unmarshal(bytes, p)
}

type projectSlice []Project

func (s projectSlice) Len() int {
	return len(s)
}

func (s projectSlice) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

func (s projectSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Save 保存配置文件
func (p *ProjectsXml) Save(filename string) error {
	sort.Sort(projectSlice(p.Projects))
	bytes, err := xml.MarshalIndent(p, "", "    ")
	if err != nil {
		return err
	}
	bytes = []byte(xml.Header + string(bytes))
	return ioutil.WriteFile(filename, bytes, os.ModePerm)
}
