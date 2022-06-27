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

package log

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	RegisterPlugin("Filters", PluginTypeFilter, (*CompositeFilter)(nil))
	RegisterPlugin("DenyAllFilter", PluginTypeFilter, (*DenyAllFilter)(nil))
	RegisterPlugin("LevelFilter", PluginTypeFilter, (*LevelFilter)(nil))
	RegisterPlugin("LevelMatchFilter", PluginTypeFilter, (*LevelMatchFilter)(nil))
	RegisterPlugin("LevelRangeFilter", PluginTypeFilter, (*LevelRangeFilter)(nil))
	RegisterPlugin("TimeFilter", PluginTypeFilter, (*TimeFilter)(nil))
}

type Result int

const (
	ResultAccept = Result(iota)
	ResultDeny
)

func ParseResult(s string) (Result, error) {
	switch strings.ToLower(s) {
	case "accept":
		return ResultAccept, nil
	case "deny":
		return ResultDeny, nil
	default:
		return -1, fmt.Errorf("invalid result %s", s)
	}
}

// Filter is an interface that tells the logger a log message should
// be dropped when the Filter method returns ResultDeny.
type Filter interface {
	LifeCycle
	Filter(level Level, e Entry, msg Message) Result
}

type BaseFilter struct {
	OnMatch    Result `PluginAttribute:"onMatch,default=accept"`
	OnMismatch Result `PluginAttribute:"onMismatch,default=deny"`
}

func (c *BaseFilter) Start() error             { return nil }
func (c *BaseFilter) Stop(ctx context.Context) {}

// CompositeFilter composes and invokes one or more filters.
type CompositeFilter struct {
	Filters []Filter `PluginElement:"Filter"`
}

func (f *CompositeFilter) Start() error {
	for _, filter := range f.Filters {
		if err := filter.Start(); err != nil {
			return err
		}
	}
	return nil
}

func (f *CompositeFilter) Stop(ctx context.Context) {
	for _, filter := range f.Filters {
		filter.Stop(ctx)
	}
}

func (f *CompositeFilter) Filter(level Level, e Entry, msg Message) Result {
	for _, filter := range f.Filters {
		if ResultDeny == filter.Filter(level, e, msg) {
			return ResultDeny
		}
	}
	return ResultAccept
}

// DenyAllFilter causes all logging events to be dropped.
type DenyAllFilter struct{}

func (f *DenyAllFilter) Start() error                              { return nil }
func (f *DenyAllFilter) Stop(ctx context.Context)                  {}
func (f *DenyAllFilter) Filter(_ Level, _ Entry, _ Message) Result { return ResultDeny }

// LevelFilter logs events if the level in the Event is same or more specific
// than the configured level.
type LevelFilter struct {
	BaseFilter
	Level Level `PluginAttribute:"level"`
}

func (f *LevelFilter) Filter(level Level, e Entry, msg Message) Result {
	if level >= f.Level {
		return f.OnMatch
	}
	return f.OnMismatch
}

// LevelMatchFilter logs events if the level in the Event matches the specified
// logging level exactly.
type LevelMatchFilter struct {
	BaseFilter
	Level Level `PluginAttribute:"level"`
}

func (f *LevelMatchFilter) Filter(level Level, e Entry, msg Message) Result {
	if level == f.Level {
		return f.OnMatch
	}
	return f.OnMismatch
}

// LevelRangeFilter logs events if the level in the Event is in the range of the
// configured min and max levels.
type LevelRangeFilter struct {
	BaseFilter
	MinLevel Level `PluginAttribute:"minLevel"`
	MaxLevel Level `PluginAttribute:"maxLevel"`
}

func (f *LevelRangeFilter) Filter(level Level, e Entry, msg Message) Result {
	if level >= f.MinLevel && level <= f.MaxLevel {
		return f.OnMatch
	}
	return f.OnMismatch
}

// TimeFilter filters events that fall within a specified time period in each day.
type TimeFilter struct {
	BaseFilter
	Timezone string `PluginAttribute:"timezone,default=Local"`
	Start    string `PluginAttribute:"start"`
	End      string `PluginAttribute:"end"`
	TimeFunc func() time.Time
	location *time.Location
	abs      time.Time
	start    int
	end      int
}

func (f *TimeFilter) Init() error {
	const layout = "15:04:05"
	if f.TimeFunc == nil {
		f.TimeFunc = time.Now
	}
	location, err := time.LoadLocation(f.Timezone)
	if err != nil {
		return err
	}
	startTime0, err := time.ParseInLocation(layout, "00:00:00", location)
	if err != nil {
		return err
	}
	endTime0, err := time.ParseInLocation(layout, "00:00:00", location)
	if err != nil {
		return err
	}
	startTime1, err := time.ParseInLocation(layout, f.Start, location)
	if err != nil {
		return err
	}
	endTime1, err := time.ParseInLocation(layout, f.End, location)
	if err != nil {
		return err
	}
	f.location = location
	f.start = int(startTime1.Sub(startTime0) / time.Second)
	f.end = int(endTime1.Sub(endTime0) / time.Second)
	f.abs = time.Date(2020, 1, 1, 0, 0, 0, 0, location)
	return nil
}

func (f *TimeFilter) Filter(level Level, e Entry, msg Message) Result {
	t := int(f.TimeFunc().Sub(f.abs)/time.Second) % 86400
	if t >= f.start && t <= f.end {
		return f.OnMatch
	}
	return f.OnMismatch
}
