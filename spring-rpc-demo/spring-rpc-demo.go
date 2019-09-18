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

package SpringRpcDemo

import (
	"runtime/debug"
	"fmt"
	"sync"
	"runtime"
	"net"
	"crypto/tls"
	"encoding/xml"
	"context"
	"github.com/didi/go-spring/spring-rpc"
	Logger "github.com/didi/go-spring/spring-logger"
)

//
// 错误值
//
type Error struct {
	Code int32  `xml:"code"`
	Msg  string `xml:"msg"`
}

var (
	ERROR   = Error{-1, "ERROR"}
	SUCCESS = Error{200, "SUCCESS"}
)

//
// 返回值类型
//
type RpcResult struct {
	XMLName xml.Name `xml:"response"`

	Error

	Data interface{} `xml:"data,omitempty"`
}

func ReadRpcResult(b []byte, i interface{}) error {
	var r RpcResult
	r.Data = i
	return xml.Unmarshal(b, &r)
}

type SpringRpcDemoContext struct {
	body []byte
}

func (ctx *SpringRpcDemoContext) Bind(i interface{}) error {
	var r DemoRequest
	r.Data = i
	return xml.Unmarshal(ctx.body, &r)
}

func (ctx *SpringRpcDemoContext) Context() context.Context {
	return context.Background()
}

type SpringRpcDemoContainer struct {
	exitChan chan struct{}
}

func (c *SpringRpcDemoContainer) Stop() {
	close(c.exitChan)
}

func (c *SpringRpcDemoContainer) Start(address string) error {
	return RunServer(address, false, "", "", c.exitChan)
}

func (c *SpringRpcDemoContainer) StartTLS(address string, certFile, keyFile string) error {
	return RunServer(address, true, certFile, keyFile, c.exitChan)
}

var serviceMap map[string]func(body []byte) interface{}

func init() {
	serviceMap = make(map[string]func(body []byte) interface{})
}

func (c *SpringRpcDemoContainer) Register(service string, method string, fn SpringRpc.Handler) {
	serviceMap[service+"#"+method] = func(body []byte) interface{} {
		data := fn(&SpringRpcDemoContext{body})
		return &RpcResult{
			Error: SUCCESS,
			Data:  data,
		}
	}
}

type DemoRequest struct {
	XMLName xml.Name    `xml:"request"`
	Service string      `xml:"service"`
	Method  string      `xml:"method"`
	Data    interface{} `xml:"data"`
}

func NewDemoRequest(service string, method string, data interface{}) *DemoRequest {
	return &DemoRequest{Service: service, Method: method, Data: data}
}

func RunServer(address string, secure bool, certFile string, keyFile string, exitChan chan struct{}) error {

	var (
		err error
		ln  net.Listener
	)

	if secure {

		var cert tls.Certificate
		if cert, err = tls.LoadX509KeyPair(certFile, keyFile); err != nil {
			return err
		}

		tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
		ln, err = tls.Listen("tcp", address, tlsConfig)

	} else {
		ln, err = net.Listen("tcp", address)
	}

	if err != nil {
		return err
	}

	newConn := make(chan net.Conn)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				Logger.Errorln(err, ";", string(debug.Stack()))
			}
		}()

		for {
			if conn, err := ln.Accept(); err != nil {
				close(exitChan)
				panic(err)
			} else {
				Logger.Infoln(fmt.Sprintf("accept new connection %s", conn.RemoteAddr()))
				newConn <- conn
			}
		}
	}()

	var (
		cn int32
		wg sync.WaitGroup // 等待子线程退出
	)

	Logger.Infoln("before server cycle - runtime.NumGoroutine =", runtime.NumGoroutine())

	for {
		select {
		case <-exitChan:
			fmt.Printf("waiting %d connections exit\n", cn)
			wg.Wait()
			fmt.Println("all connections exited")
			return nil
		case conn := <-newConn:
			go func() {
				count := make([]byte, 1)
				conn.Read(count)

				body := make([]byte, count[0])
				conn.Read(body)

				var r DemoRequest
				xml.Unmarshal(body, &r)

				res := serviceMap[r.Service+"#"+r.Method](body)

				b, _ := xml.MarshalIndent(&res, "", "  ")
				fmt.Println(string(b))

				conn.Write([]byte{byte(len(b))})
				conn.Write(b)
			}()
		}
	}
}

func CallService(service string, method string, reqData interface{}, respData interface{}) error {

	r := NewDemoRequest(service, method, reqData)

	b, _ := xml.MarshalIndent(&r, "", "  ")
	fmt.Println(string(b))

	conn, _ := net.Dial("tcp", ":8080")
	conn.Write([]byte{byte(len(b))})
	conn.Write(b)

	count := make([]byte, 1)
	conn.Read(count)

	body := make([]byte, count[0])
	conn.Read(body)

	return ReadRpcResult(body, respData)
}
