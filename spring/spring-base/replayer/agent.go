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

package replayer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/recorder"
)

func init() {
	runAgent()
}

var agentCfg = AgentConfig{
	Port:    8991,
	DataDir: "replay",
}

type AgentConfig struct {
	Port    int
	DataDir string
}

func SetAgentConfig(config AgentConfig) {
	agentCfg = config
}

func runAgent() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", home)
	mux.HandleFunc("/replay", replay)
	go func() {
		addr := fmt.Sprintf(":%d", agentCfg.Port)
		fmt.Println(http.ListenAndServe(addr, mux))
	}()
}

func checkDataDir(w http.ResponseWriter, r *http.Request) (ok bool) {

	defer func() {
		if !ok {
			w.Write([]byte(fmt.Sprintf("error replay data dir %q", agentCfg.DataDir)))
		}
	}()

	if agentCfg.DataDir == "" {
		return false
	}

	dir, err := filepath.Abs(agentCfg.DataDir)
	if err != nil {
		return false
	}

	stat, err := os.Stat(dir)
	if err != nil {
		return false
	}
	return stat.IsDir()
}

func readDirNames(dirname string) ([]string, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	names, err := f.Readdirnames(-1)
	f.Close()
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

func home(w http.ResponseWriter, r *http.Request) {

	if !checkDataDir(w, r) {
		return
	}

	names, err := readDirNames(agentCfg.DataDir)
	if err != nil {
		w.Write([]byte(fmt.Sprintf("read %q names error %s", agentCfg.DataDir, err.Error())))
		return
	}

	var buf bytes.Buffer
	for _, name := range names {
		buf.WriteString(name)
		buf.WriteByte('\n')
	}
	w.Write(buf.Bytes())
}

func replay(w http.ResponseWriter, r *http.Request) {

	if !checkDataDir(w, r) {
		return
	}

	if err := r.ParseForm(); err != nil {
		w.Write([]byte(fmt.Sprintf("parse form error %s", err.Error())))
		return
	}

	id := r.Form.Get("id")
	if id == "" {
		w.Write([]byte(fmt.Sprintf("error replay id %s", id)))
		return
	}

	dataFile := filepath.Join(agentCfg.DataDir, id+".data")
	info, err := os.Stat(dataFile)
	if err != nil {
		w.Write([]byte(fmt.Sprintf("stat file %s error %s", dataFile, err.Error())))
		return
	}
	if info.IsDir() {
		w.Write([]byte(fmt.Sprintf("file %s is directory", dataFile)))
		return
	}

	data, err := ioutil.ReadFile(dataFile)
	if err != nil {
		w.Write([]byte(fmt.Sprintf("read file %s error %s", dataFile, err.Error())))
		return
	}

	session := new(recorder.Session)
	if err = json.Unmarshal(data, &session); err != nil {
		w.Write([]byte(fmt.Sprintf("unmarshal file %s error %s", dataFile, err.Error())))
		return
	}

	w.Write([]byte(cast.ToString(session)))

	if len(session.Actions) == 0 {
		w.Write([]byte(fmt.Sprintf("actions not found in file %s", dataFile)))
		return
	}

	action := session.Actions[0]
	if action.Protocol != recorder.HTTP {
		w.Write([]byte(fmt.Sprintf("first action not http in file %s", dataFile)))
		return
	}

	Store(session.ID, session)
	defer Delete(session.ID)

	//httpData := action.Data.(*recorder.Http)
	url := "http://127.0.0.1:8080/index"
	resp, err := http.Get(url)
	if err != nil {
		w.Write([]byte(fmt.Sprintf("post form %s return error %s", url, err.Error())))
		return
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	fmt.Println(string(body))
}
