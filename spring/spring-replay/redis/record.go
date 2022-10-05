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

package redis

//
//type recordConn struct {
//	conn ConnPool
//}
//
//func (c *recordConn) Exec(ctx context.Context, cmd string, args []interface{}) (ret interface{}, err error) {
//	defer func() {
//		recorder.RecordAction(ctx, recorder.REDIS, &recorder.SimpleAction{
//			Request: func() string {
//				return recorder.EncodeTTY(append([]interface{}{cmd}, args...)...)
//			},
//			Response: func() string {
//				if err == nil {
//					return recorder.EncodeCSV(ret)
//				}
//				if IsErrNil(err) {
//					return "NULL"
//				}
//				return "(err) " + err.Error()
//			},
//		})
//	}()
//	return c.conn.Exec(ctx, cmd, args)
//}
