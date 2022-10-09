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
	"errors"
	"fmt"
	"strings"
	"time"
)

func init() {
	RegisterPlugin("AcceptAllFilter", PluginTypeFilter, (Filter)((*AcceptAllFilter)(nil)))
	RegisterPlugin("DenyAllFilter", PluginTypeFilter, (Filter)((*DenyAllFilter)(nil)))
	RegisterPlugin("LevelFilter", PluginTypeFilter, (Filter)((*LevelFilter)(nil)))
	RegisterPlugin("LevelMatchFilter", PluginTypeFilter, (Filter)((*LevelMatchFilter)(nil)))
	RegisterPlugin("LevelRangeFilter", PluginTypeFilter, (Filter)((*LevelRangeFilter)(nil)))
	RegisterPlugin("TimeFilter", PluginTypeFilter, (Filter)((*TimeFilter)(nil)))
	RegisterPlugin("TagFilter", PluginTypeFilter, (Filter)((*TagFilter)(nil)))
	RegisterPlugin("Filters", PluginTypeFilter, (Filter)((*CompositeFilter)(nil)))
}

type Result int

const (
	ResultAccept = Result(iota)
	ResultDeny
)

// Filter is an interface that tells the logger a log message should
// be dropped when the Filter method returns ResultDeny.
// Filter 只应该出现在两个地方，一个是 Logger 上，用于控制消息是否打印，另一个是
// AppenderRef，用于控制消息是否输出到 Appender 上，即控制消息路由。
type Filter interface {
	Filter(e *Event) Result
}

// AcceptAllFilter causes all logging events to be accepted.
type AcceptAllFilter struct{}

func (f *AcceptAllFilter) Filter(e *Event) Result {
	return ResultAccept
}

// DenyAllFilter causes all logging events to be dropped.
type DenyAllFilter struct{}

func (f *DenyAllFilter) Filter(e *Event) Result {
	return ResultDeny
}

// LevelFilter logs events if the level in the Event is same or more specific
// than the configured level.
type LevelFilter struct {
	Level Level `PluginAttribute:"level"`
}

func (f *LevelFilter) Filter(e *Event) Result {
	if e.Level >= f.Level {
		return ResultAccept
	}
	return ResultDeny
}

// LevelMatchFilter logs events if the level in the Event matches the specified
// logging level exactly.
type LevelMatchFilter struct {
	Level Level `PluginAttribute:"level"`
}

func (f *LevelMatchFilter) Filter(e *Event) Result {
	if e.Level == f.Level {
		return ResultAccept
	}
	return ResultDeny
}

// LevelRangeFilter logs events if the level in the Event is in the range of the
// configured min and max levels.
type LevelRangeFilter struct {
	Min Level `PluginAttribute:"min"`
	Max Level `PluginAttribute:"max"`
}

func (f *LevelRangeFilter) Filter(e *Event) Result {
	if e.Level >= f.Min && e.Level <= f.Max {
		return ResultAccept
	}
	return ResultDeny
}

// TimeFilter filters events that fall within a specified time period in each day.
type TimeFilter struct {
	Timezone string `PluginAttribute:"timezone,default=Local"`
	Start    string `PluginAttribute:"start"`
	End      string `PluginAttribute:"end"`
	location *time.Location
	abs      time.Time
	start    int
	end      int
}

func (f *TimeFilter) Init() error {
	const layout = "15:04:05"
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

func (f *TimeFilter) Filter(e *Event) Result {
	t := int(e.Time.Sub(f.abs)/time.Second) % 86400
	if t >= f.start && t <= f.end {
		return ResultAccept
	}
	return ResultDeny
}

type TagFilter struct {
	Prefix string `PluginAttribute:"prefix,default="`
	Suffix string `PluginAttribute:"suffix,default="`
	Tag    string `PluginAttribute:"tag,default="`
	tags   []string
}

func (f *TagFilter) Init() error {
	if f.Prefix == "" && f.Suffix == "" && f.Tag == "" {
		return errors.New("TagFilter needs tag/prefix/suffix attribute")
	}
	f.tags = strings.Split(f.Tag, ",")
	return nil
}

func (f *TagFilter) Filter(e *Event) Result {
	if f.Prefix != "" && strings.HasPrefix(e.Tag, f.Prefix) {
		return ResultAccept
	}
	if f.Suffix != "" && strings.HasSuffix(e.Tag, f.Suffix) {
		return ResultAccept
	}
	for _, tag := range f.tags {
		if e.Tag == tag {
			return ResultAccept
		}
	}
	return ResultDeny
}

type Operator int

const (
	OperatorAnd Operator = iota
	OperatorOr
	OperatorNone
)

func ParseOperator(s string) (Operator, error) {
	switch strings.ToLower(s) {
	case "and":
		return OperatorAnd, nil
	case "or":
		return OperatorOr, nil
	case "none":
		return OperatorNone, nil
	default:
		return -1, fmt.Errorf("invalid operator '%s'", s)
	}
}

type CompositeFilter struct {
	Filters  []Filter `PluginElement:"Filter"`
	Operator Operator //`PluginAttribute:"operator,default=and"`
}

func (f *CompositeFilter) Start() error {
	for _, filter := range f.Filters {
		if v, ok := filter.(LifeCycle); ok {
			if err := v.Start(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *CompositeFilter) Stop(ctx context.Context) {
	for _, filter := range f.Filters {
		if v, ok := filter.(LifeCycle); ok {
			v.Stop(ctx)
		}
	}
}

func (f *CompositeFilter) Filter(e *Event) Result {
	switch f.Operator {
	case OperatorAnd:
		for _, filter := range f.Filters {
			if ResultDeny == filter.Filter(e) {
				return ResultDeny
			}
		}
		return ResultAccept
	case OperatorOr:
		for _, filter := range f.Filters {
			if ResultAccept == filter.Filter(e) {
				return ResultAccept
			}
		}
		return ResultDeny
	case OperatorNone:
		for _, filter := range f.Filters {
			if ResultAccept == filter.Filter(e) {
				return ResultDeny
			}
		}
		return ResultAccept
	}
	return ResultAccept
}
