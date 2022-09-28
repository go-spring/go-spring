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

package cache_test

//
//func init() {
//
//	config := `
//		<?xml version="1.0" encoding="UTF-8"?>
//		<Configuration>
//			<Appenders>
//				<Console name="Console"/>
//			</Appenders>
//			<Loggers>
//				<Root level="info">
//					<AppenderRef ref="Console"/>
//				</Root>
//			</Loggers>
//		</Configuration>
//	`
//	err := log.RefreshBuffer(config, ".xml")
//	util.Panic(err).When(err != nil)
//
//	recorder.Init()
//	recorder.RegisterProtocol(recorder.REDIS, &redisProtocol{})
//}
//
//type redisProtocol struct{}
//
//func (p *redisProtocol) ShouldDiff() bool {
//	return true
//}
//
//func (p *redisProtocol) GetLabel(data string) string {
//	return data[:4]
//}
//
//func (p *redisProtocol) FlatRequest(data string) (map[string]string, error) {
//	return nil, nil
//}
//
//func (p *redisProtocol) FlatResponse(data string) (map[string]string, error) {
//	return nil, nil
//}
//
//type response struct {
//	Name string `json:"name"`
//}
//
//type redis struct{}
//
//func (r *redis) getValue(ctx context.Context, key string) (ret string, err error) {
//	defer func() {
//		if recorder.RecordMode() {
//			recorder.RecordAction(ctx, recorder.REDIS, &recorder.SimpleAction{
//				Request: func() string {
//					return key
//				},
//				Response: func() string {
//					return ret
//				},
//			})
//		}
//	}()
//	if replayer.ReplayMode() {
//		var (
//			ok   bool
//			resp interface{}
//		)
//		resp, ok, err = replayer.Query(ctx, recorder.REDIS, key)
//		if err != nil {
//			return "", err
//		}
//		if !ok {
//			return "", errors.New("no replay data")
//		}
//		return resp.(string), nil
//	}
//	return fmt.Sprintf("{\"name\":\"%s\"}", key), nil
//}
//
//func getResponse(ctx context.Context, r *redis, key string) (*response, cache.LoadType, error) {
//	loadType, result, err := cache.Load(ctx, key, func(ctx context.Context, key string) (interface{}, error) {
//		data, err := r.getValue(ctx, key)
//		if err != nil {
//			return nil, err
//		}
//		var v *response
//		err = json.Unmarshal([]byte(data), &v)
//		if err != nil {
//			return nil, err
//		}
//		return v, nil
//	}, cache.ExpireAfterWrite(0))
//	if err != nil {
//		return nil, cache.LoadNone, err
//	}
//	var resp *response
//	err = result.Load(&resp)
//	if err != nil {
//		return nil, cache.LoadNone, err
//	}
//	return resp, loadType, nil
//}
//
//func testFunc(t *testing.T, ctx context.Context, key string) []cache.LoadType {
//
//	var (
//		ret  []cache.LoadType
//		lock sync.Mutex
//	)
//
//	r := &redis{}
//	wg := sync.WaitGroup{}
//	for i := 0; i < 3; i++ {
//		wg.Add(1)
//		go func() {
//			defer wg.Done()
//			resp, loadType, err := getResponse(ctx, r, key)
//			lock.Lock()
//			ret = append(ret, loadType)
//			lock.Unlock()
//			assert.Nil(t, err)
//			fmt.Printf("%v %#v\n", loadType, resp)
//		}()
//	}
//	wg.Wait()
//
//	return ret
//}
//
//type SessionSlice []*recorder.Session
//
//func (p SessionSlice) Len() int           { return len(p) }
//func (p SessionSlice) Less(i, j int) bool { return p[i].Actions[0].Protocol > p[j].Actions[0].Protocol }
//func (p SessionSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
//
//type LoadTypeSlice []cache.LoadType
//
//func (p LoadTypeSlice) Len() int           { return len(p) }
//func (p LoadTypeSlice) Less(i, j int) bool { return p[i] > p[j] }
//func (p LoadTypeSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
//
//type LoadTypeSliceSlice [][]cache.LoadType
//
//func (p LoadTypeSliceSlice) Len() int           { return len(p) }
//func (p LoadTypeSliceSlice) Less(i, j int) bool { return p[i][0] > p[j][0] }
//func (p LoadTypeSliceSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
//
//func TestRecord(t *testing.T) {
//	defer cache.InvalidateAll()
//
//	recorder.SetRecordMode(true)
//	defer func() {
//		recorder.SetRecordMode(false)
//	}()
//
//	key := "test"
//	f := func(sessionID string) (*recorder.Session, []cache.LoadType) {
//		ctx, _ := knife.New(context.Background())
//		recorder.StartRecord(ctx, func() (string, error) {
//			return sessionID, nil
//		})
//		loadTypes := testFunc(t, ctx, key)
//		session := recorder.StopRecord(ctx)
//		fmt.Println(recorder.ToPrettyJson(session))
//		return session, loadTypes
//	}
//
//	var (
//		loadTypes [][]cache.LoadType
//		sessions  []*recorder.Session
//		lock      sync.Mutex
//	)
//
//	var wg sync.WaitGroup
//	for i := 0; i < 3; i++ {
//		sessionID := strconv.Itoa(i)
//		wg.Add(1)
//		go func() {
//			defer wg.Done()
//			s1, s2 := f(sessionID)
//			lock.Lock()
//			sessions = append(sessions, s1)
//			loadTypes = append(loadTypes, s2)
//			lock.Unlock()
//		}()
//	}
//	wg.Wait()
//
//	for i := 0; i < 3; i++ {
//		sort.Sort(LoadTypeSlice(loadTypes[i]))
//	}
//	sort.Sort(LoadTypeSliceSlice(loadTypes))
//
//	assert.Equal(t, loadTypes, [][]cache.LoadType{
//		{cache.LoadBack, cache.LoadOnCtx, cache.LoadOnCtx},
//		{cache.LoadCache, cache.LoadOnCtx, cache.LoadOnCtx},
//		{cache.LoadCache, cache.LoadOnCtx, cache.LoadOnCtx},
//	})
//
//	sort.Sort(SessionSlice(sessions))
//
//	var ss []string
//	for _, s := range sessions {
//		s.Session = ""
//		ss = append(ss, recorder.ToJson(s))
//	}
//
//	//configs := []*jsondiff.Config{
//	//	jsondiff.Path("$..[Timestamp]").SetComparator(func(a, b interface{}) bool {
//	//		return true
//	//	}),
//	//}
//	//r := jsondiff.Diff(ss[0], `{"Actions":[{"Protocol":"REDIS","Request":"test","Response":"{\"name\":\"test\"}"}]}`, configs...)
//	//assert.Equal(t, r.Differs, map[string]jsondiff.DiffItem{})
//	//jsondiff.Diff(ss[1], `{"Actions":[{"Protocol":"CACHE","Request":"test","Response":"{\"name\":\"test\"}"}]}`, configs...)
//	//assert.Equal(t, r.Differs, map[string]jsondiff.DiffItem{})
//	//jsondiff.Diff(ss[2], `{"Actions":[{"Protocol":"CACHE","Request":"test","Response":"{\"name\":\"test\"}"}]}`, configs...)
//	//assert.Equal(t, r.Differs, map[string]jsondiff.DiffItem{})
//}
//
//func TestReplay(t *testing.T) {
//	defer cache.InvalidateAll()
//
//	replayer.SetReplayMode(true)
//	defer func() {
//		replayer.SetReplayMode(false)
//	}()
//
//	agent := replayer.NewLocalAgent()
//	replayer.SetReplayAgent(agent)
//
//	key := "test"
//	f := func(str string) []cache.LoadType {
//
//		session, err := agent.Store(str)
//		assert.Nil(t, err)
//		defer agent.Delete(session.Session)
//
//		ctx, cached := knife.New(context.Background())
//		assert.False(t, cached)
//
//		timeNow := time.Unix(1643364150, 0)
//		err = clock.SetBaseTime(ctx, timeNow)
//		assert.Nil(t, err)
//
//		err = replayer.SetSessionID(ctx, session.Session)
//		assert.Nil(t, err)
//
//		return testFunc(t, ctx, key)
//	}
//
//	sessions := []string{
//		`{
//		  "Session": "0",
//		  "Actions": [
//			{
//			  "Protocol": "REDIS",
//			  "Request": "test",
//			  "Response": "{\"name\":\"test\"}"
//			}
//		  ]
//		}`,
//		`{
//		  "Session": "1",
//		  "Actions": [
//			{
//			  "Protocol": "CACHE",
//			  "Request": "test",
//			  "Response": "{\"name\":\"test\"}"
//			}
//		  ]
//		}`,
//		`{
//		  "Session": "2",
//		  "Actions": [
//			{
//			  "Protocol": "CACHE",
//			  "Request": "test",
//			  "Response": "{\"name\":\"test\"}"
//			}
//		  ]
//		}`,
//	}
//
//	var (
//		loadTypes [][]cache.LoadType
//		lock      sync.Mutex
//	)
//
//	var wg sync.WaitGroup
//	for i := 0; i < 3; i++ {
//		j := i
//		wg.Add(1)
//		go func() {
//			defer wg.Done()
//			s2 := f(sessions[j])
//			lock.Lock()
//			loadTypes = append(loadTypes, s2)
//			lock.Unlock()
//		}()
//	}
//	wg.Wait()
//
//	for i := 0; i < 3; i++ {
//		sort.Sort(LoadTypeSlice(loadTypes[i]))
//	}
//	sort.Sort(LoadTypeSliceSlice(loadTypes))
//
//	assert.Equal(t, loadTypes, [][]cache.LoadType{
//		{cache.LoadBack, cache.LoadOnCtx, cache.LoadOnCtx},
//		{cache.LoadCache, cache.LoadOnCtx, cache.LoadOnCtx},
//		{cache.LoadCache, cache.LoadOnCtx, cache.LoadOnCtx},
//	})
//}
