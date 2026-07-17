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

	"go-spring.org/spring/gs"
	"gorm.io/gorm"
)

func init() {
	gs.Provide(&demoRunner{}).Export(gs.As[gs.Runner]())
}

// Widget is a trivial table the runner creates and populates so the GORM otel
// plugin emits a handful of create/query spans and reports pool metrics.
type Widget struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

// demoRunner exercises the database once at startup. The *gorm.DB is the single
// bean produced by starter-gorm-mysql (autowired by type). Because starter-otel
// set the OTel globals before this bean was constructed, the plugin the gorm
// starter installed is already exporting to the collector by the time these
// statements run.
type demoRunner struct {
	DB *gorm.DB `autowire:""`
}

// Run performs schema init + a few writes + a read, each carrying ctx so the
// spans chain correctly. It returns promptly; gs.Run then blocks on signal, and
// graceful shutdown flushes the buffered spans/metrics to the collector.
func (r *demoRunner) Run(ctx context.Context) error {
	if err := r.DB.WithContext(ctx).AutoMigrate(&Widget{}); err != nil {
		return err
	}
	for i := 0; i < 5; i++ {
		w := Widget{Name: fmt.Sprintf("widget-%d", i)}
		if err := r.DB.WithContext(ctx).Create(&w).Error; err != nil {
			return err
		}
	}
	var widgets []Widget
	if err := r.DB.WithContext(ctx).Find(&widgets).Error; err != nil {
		return err
	}
	fmt.Printf("observability-gorm: inserted and read back %d widgets\n", len(widgets))
	return nil
}
