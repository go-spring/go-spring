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

package fastdev

//
//// AgentConfig 回放代理的配置。
//type AgentConfig struct {
//	Port    int
//	DataDir string
//}
//
//var agent = struct {
//	server *http.Server
//	config AgentConfig
//}{
//	config: AgentConfig{
//		Port:    8991,
//		DataDir: "replay",
//	},
//}
//
//// startAgent 启动回放代理。
//func startAgent() {
//	c := new(controller)
//	mux := http.NewServeMux()
//	mux.HandleFunc("/", c.home)
//	mux.HandleFunc("/replay", c.replay)
//	go func() {
//		addr := fmt.Sprintf(":%d", agent.config.Port)
//		agent.server = &http.Server{Addr: addr, Handler: mux}
//		fmt.Println(agent.server.ListenAndServe())
//	}()
//}
//
//// stopAgent 停止回放代理。
//func stopAgent() {
//	if agent.server != nil {
//		agent.server.Shutdown(context.Background())
//	}
//}
//
//// SetAgentConfig 设置回放代理的配置。
//func SetAgentConfig(config AgentConfig) {
//	agent.config = config
//}
//
//type controller struct{}
//
//// checkDataDir 检查回放目录是否有效。
//func checkDataDir(w http.ResponseWriter, r *http.Request) (ok bool) {
//
//	defer func() {
//		if !ok {
//			w.Write([]byte(fmt.Sprintf("error replay data dir %q", agent.config.DataDir)))
//		}
//	}()
//
//	if agent.config.DataDir == "" {
//		return false
//	}
//
//	dir, err := filepath.Abs(agent.config.DataDir)
//	if err != nil {
//		return false
//	}
//
//	stat, err := os.Stat(dir)
//	if err != nil {
//		return false
//	}
//	return stat.IsDir()
//}
//
//// readDirNames 读取回放文件列表。
//func readDirNames(dirname string) ([]string, error) {
//	f, err := os.Open(dirname)
//	if err != nil {
//		return nil, err
//	}
//	names, err := f.Readdirnames(-1)
//	f.Close()
//	if err != nil {
//		return nil, err
//	}
//	sort.Strings(names)
//	return names, nil
//}
//
//func (c *controller) home(w http.ResponseWriter, r *http.Request) {
//
//	if !checkDataDir(w, r) {
//		return
//	}
//
//	names, err := readDirNames(agent.config.DataDir)
//	if err != nil {
//		w.Write([]byte(fmt.Sprintf("read %q names error %s", agent.config.DataDir, err.Error())))
//		return
//	}
//
//	w.Write([]byte(strings.Join(names, "\n")))
//}
//
//func (c *controller) replay(w http.ResponseWriter, r *http.Request) {
//
//	if !checkDataDir(w, r) {
//		return
//	}
//
//	if err := r.ParseForm(); err != nil {
//		w.Write([]byte(fmt.Sprintf("parse form error %s", err.Error())))
//		return
//	}
//
//	sessionID := r.Form.Get("session")
//	if sessionID == "" {
//		w.Write([]byte(fmt.Sprintf("empty session id %s", sessionID)))
//		return
//	}
//
//	dataFile := filepath.Join(agent.config.DataDir, sessionID+".data")
//	info, err := os.Stat(dataFile)
//	if err != nil {
//		w.Write([]byte(fmt.Sprintf("stat file %s error %s", dataFile, err.Error())))
//		return
//	}
//	if info.IsDir() {
//		w.Write([]byte(fmt.Sprintf("file %s is directory", dataFile)))
//		return
//	}
//
//	data, err := ioutil.ReadFile(dataFile)
//	if err != nil {
//		w.Write([]byte(fmt.Sprintf("read file %s error %s", dataFile, err.Error())))
//		return
//	}
//
//	var session *Session
//	if err = json.Unmarshal(data, &session); err != nil {
//		w.Write([]byte(fmt.Sprintf("unmarshal file %s error %s", dataFile, err.Error())))
//		return
//	}
//
//	if session.Inbound == nil {
//		w.Write([]byte(fmt.Sprintf("inbound not found in file %s", dataFile)))
//		return
//	}
//
//	if session.Inbound.Protocol != HTTP {
//		w.Write([]byte(fmt.Sprintf("inbound not http in file %s", dataFile)))
//		return
//	}
//
//	Store(session)
//	defer Delete(session.Session)
//
//	if err = c.replaySession(session); err != nil {
//		w.Write([]byte(fmt.Sprintf("replay file %s error %s", dataFile, err.Error())))
//		return
//	}
//
//	w.Write([]byte(cast.ToString(session)))
//}
//
//func (c *controller) replaySession(session *Session) error {
//
//	reqBuf := bytes.NewBufferString(session.Inbound.Request.(string))
//	req, err := http.ReadRequest(bufio.NewReader(reqBuf))
//	if err != nil {
//		return err
//	}
//
//	req.RequestURI = ""
//	req.URL.Scheme = "http"
//	req.URL.Host = req.Host
//
//	req.Header.Set("REPLAY-SESSION-ID", session.Session)
//	resp, err := http.DefaultClient.Do(req)
//	if err != nil {
//		return err
//	}
//
//	defer resp.Body.Close()
//
//	body, err := ioutil.ReadAll(resp.Body)
//	if err != nil {
//		return fmt.Errorf("read url %s data return error %s", req.URL, err.Error())
//	}
//
//	fmt.Println(string(body))
//	return nil
//}
