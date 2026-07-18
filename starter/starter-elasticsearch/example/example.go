/*
 * Copyright 2025 The Go-Spring Authors.
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
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterElasticsearch "go-spring.org/starter-elasticsearch"
)

func init() {
	StarterElasticsearch.RegisterDriver("AnotherESDriver", &AnotherESDriver{})
}

// AnotherESDriver is a custom implementation of the Driver interface.
type AnotherESDriver struct{}

func (AnotherESDriver) CreateClient(c StarterElasticsearch.Config) (*elasticsearch.Client, error) {
	log.Infof(context.Background(), log.TagAppDef, "AnotherESDriver::CreateClient")
	return elasticsearch.NewClient(elasticsearch.Config{
		Addresses: c.Addresses,
		Username:  c.Username,
		Password:  c.Password,
	})
}

const indexName = "starter-es-example"

type Service struct {
	ES *elasticsearch.Client `autowire:"docs"`
}

func main() {
	// You can change the `driver` property in the configuration file
	// and check the used Elasticsearch driver via logs.

	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	// Define a handler to index a document.
	http.HandleFunc("/index", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		body := `{"title":"hello","views":1}`
		res, err := s.ES.Index(indexName, strings.NewReader(body),
			s.ES.Index.WithDocumentID("1"),
			s.ES.Index.WithRefresh("true"))
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		defer func() { _ = res.Body.Close() }()
		_, _ = io.Copy(w, res.Body)
	})

	// Define a handler to get a document by ID.
	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		res, err := s.ES.Get(indexName, "1")
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		defer func() { _ = res.Body.Close() }()
		_, _ = io.Copy(w, res.Body)
	})

	// Define a handler to search documents.
	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		query := `{"query":{"match":{"title":"hello"}}}`
		res, err := s.ES.Search(
			s.ES.Search.WithIndex(indexName),
			s.ES.Search.WithBody(strings.NewReader(query)))
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		defer func() { _ = res.Body.Close() }()
		_, _ = io.Copy(w, res.Body)
	})

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest(svrBean.Interface().(*Service))
	}()

	// Run the Go-Spring application.
	gs.Run()

	// Example usage:
	//
	// ~ curl http://127.0.0.1:9090/index
	// ~ curl http://127.0.0.1:9090/get
	// ~ curl http://127.0.0.1:9090/search
}

func runTest(s *Service) {
	ctx := context.Background()

	// Feature 1: readiness probe — verify cluster connectivity.
	if err := StarterElasticsearch.HealthCheck(s.ES); err != nil {
		log.Errorf(ctx, log.TagAppDef, "HealthCheck failed: %v", err)
		os.Exit(1)
	}

	// Feature 2: index a document (refresh so it is immediately searchable).
	body := `{"title":"hello","views":1}`
	idxRes, err := s.ES.Index(indexName, strings.NewReader(body),
		s.ES.Index.WithDocumentID("1"),
		s.ES.Index.WithRefresh("true"))
	if err != nil || idxRes.IsError() {
		log.Errorf(ctx, log.TagAppDef, "Index failed: err=%v res=%v", err, idxRes)
		os.Exit(1)
	}
	_ = idxRes.Body.Close()

	// Feature 3: get the document back by ID.
	getRes, err := s.ES.Get(indexName, "1")
	if err != nil || getRes.IsError() {
		log.Errorf(ctx, log.TagAppDef, "Get failed: err=%v res=%v", err, getRes)
		os.Exit(1)
	}
	getBody, _ := io.ReadAll(getRes.Body)
	_ = getRes.Body.Close()
	if !strings.Contains(string(getBody), `"found":true`) {
		log.Errorf(ctx, log.TagAppDef, "Get did not find the document: %s", getBody)
		os.Exit(1)
	}

	// Feature 4: search the document by matching its title.
	query := `{"query":{"match":{"title":"hello"}}}`
	searchRes, err := s.ES.Search(
		s.ES.Search.WithIndex(indexName),
		s.ES.Search.WithBody(strings.NewReader(query)))
	if err != nil || searchRes.IsError() {
		log.Errorf(ctx, log.TagAppDef, "Search failed: err=%v res=%v", err, searchRes)
		os.Exit(1)
	}
	searchBody, _ := io.ReadAll(searchRes.Body)
	_ = searchRes.Body.Close()
	if !strings.Contains(string(searchBody), `"title":"hello"`) {
		log.Errorf(ctx, log.TagAppDef, "Search did not return the document: %s", searchBody)
		os.Exit(1)
	}

	fmt.Println("Response from server: indexed, get found, search matched")
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory of the application to the directory
// where this source file resides.
// This ensures that any relative file operations are based on the source file location,
// not the process launch path.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	err := os.Chdir(execDir)
	if err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
