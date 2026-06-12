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

package resolving

import (
	"reflect"
	"regexp"
	"slices"

	"github.com/go-spring/spring-core/gs/internal/gs"
	"github.com/go-spring/spring-core/gs/internal/gs_bean"
	"github.com/go-spring/spring-core/gs/internal/gs_cond"
	"github.com/go-spring/spring-core/gs/internal/gs_init"
	"github.com/go-spring/stdlib/errutil"
	"github.com/go-spring/stdlib/flatten"
	"github.com/go-spring/stdlib/funcutil"
)

// RefreshState represents the current state of the container.
type RefreshState int

const (
	RefreshDefault = RefreshState(iota)
	RefreshPrepare
	Refreshing
	Refreshed
)

// Resolving is the core container managing BeanDefinitions.
// It supports registering beans, applying modules, scanning configuration beans,
// resolving conditional beans, and checking for duplicates.
type Resolving struct {
	state RefreshState              // current refresh state
	beans []*gs_bean.BeanDefinition // all beans managed by the container
}

// New creates an empty Resolving instance.
func New() *Resolving {
	return &Resolving{}
}

// Beans returns all bean definitions that are not marked as deleted (StatusDeleted).
func (c *Resolving) Beans() []*gs_bean.BeanDefinition {
	var beans []*gs_bean.BeanDefinition
	for _, b := range c.beans {
		if b.GetStatus() == gs_bean.StatusDeleted {
			continue
		}
		beans = append(beans, b)
	}
	return beans
}

// Provide registers a new bean definition in the container.
// - objOrCtor can be an existing instance or a constructor function.
// - Panics if the container is already Refreshing or Refreshed.
// - Returns the BeanDefinition with caller information populated.
func (c *Resolving) Provide(objOrCtor any, args ...gs.Arg) *gs_bean.BeanDefinition {
	if c.state >= Refreshing {
		panic("container is already refreshing or refreshed")
	}
	b := gs_bean.NewBean(objOrCtor, args...)
	c.beans = append(c.beans, b)
	return b.Caller(2)
}

// Refresh performs the full container initialization lifecycle.
// Steps:
// 1. Merge globally registered beans and container beans.
// 2. Apply registered modules that satisfy their conditions.
// 3. Set the container state to Refreshing.
// 4. Scan configuration beans and register eligible methods as beans.
// 5. Resolve all beans against their conditions, marking inactive ones as deleted.
// 6. Check for duplicate beans by type and name.
// 7. Set the container state to Refreshed.
func (c *Resolving) Refresh(p flatten.Storage) error {
	if c.state != RefreshDefault {
		return errutil.Explain(nil, "container is already refreshing or refreshed")
	}
	c.state = RefreshPrepare

	c.beans = append(gs_init.Beans(), c.beans...)
	if err := c.applyModules(p); err != nil {
		return err
	}

	c.state = Refreshing

	if err := c.scanConfigurations(); err != nil {
		return err
	}

	if err := c.resolveBeans(p); err != nil {
		return err
	}

	if err := c.checkDuplicateBeans(); err != nil {
		return err
	}

	c.state = Refreshed
	return nil
}

// applyModules iterates over all globally registered modules and executes
// those whose conditions match the given context.
func (c *Resolving) applyModules(p flatten.Storage) error {
	ctx := &ConditionContext{p: p, c: c}
	for _, m := range gs_init.Modules() {
		if m.Condition != nil {
			if ok, err := m.Condition.Matches(ctx); err != nil {
				return errutil.Explain(err, "failed to apply module at %s", m.FileLine)
			} else if !ok {
				continue
			}
		}
		if err := m.ModuleFunc(c, p); err != nil {
			return errutil.Explain(err, "failed to apply module at %s", m.FileLine)
		}
	}
	return nil
}

// scanConfigurations iterates over all beans with a non-nil configuration.
// For each configuration bean, its methods are scanned to register new beans.
// Newly discovered beans are appended to the container's bean list.
func (c *Resolving) scanConfigurations() error {
	tempBeans := c.beans
	for _, b := range tempBeans {
		if b.GetConfiguration() == nil {
			continue
		}
		beans, err := c.scanConfiguration(b)
		if err != nil {
			return errutil.Explain(err, "failed to scan configuration bean [%s]", b)
		}
		c.beans = append(c.beans, beans...)
	}
	return nil
}

// scanConfiguration scans the methods of a configuration bean (bd) and
// registers methods as new beans according to include/exclude regex patterns.
//   - If Includes is empty, defaults to methods matching "New*".
//   - Methods matching any Exclude pattern are skipped.
//   - Each registered bean gets a name "<ConfigBeanName>_<MethodName>"
//     and a condition OnBeanID of the configuration bean.
func (c *Resolving) scanConfiguration(bd *gs_bean.BeanDefinition) ([]*gs_bean.BeanDefinition, error) {
	var (
		includes []*regexp.Regexp
		excludes []*regexp.Regexp
	)

	param := bd.GetConfiguration()
	ss := param.Includes
	if len(ss) == 0 {
		ss = []string{"New.*"}
	}
	for _, s := range ss {
		p, err := regexp.Compile(s)
		if err != nil {
			return nil, errutil.Explain(err, "invalid regexp '%s'", s)
		}
		includes = append(includes, p)
	}

	ss = param.Excludes
	for _, s := range ss {
		p, err := regexp.Compile(s)
		if err != nil {
			return nil, errutil.Explain(err, "invalid regexp '%s'", s)
		}
		excludes = append(excludes, p)
	}

	var ret []*gs_bean.BeanDefinition
	n := bd.GetType().NumMethod()
	for i := range n {
		m := bd.GetType().Method(i)

		// Skip methods matching any exclusion pattern.
		skip := false
		for _, p := range excludes {
			if p.MatchString(m.Name) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// Register method as a bean if it matches inclusion pattern.
		for _, p := range includes {
			if !p.MatchString(m.Name) {
				continue
			}
			b := gs_bean.NewBean(m.Func.Interface(), bd).
				Name(bd.GetName() + "_" + m.Name).
				Condition(gs_cond.OnBeanID(bd.BeanID()))
			file, line, _ := funcutil.FileLine(m.Func.Interface())
			b.SetFileLine(file, line)
			ret = append(ret, b)
			break
		}
	}
	return ret, nil
}

// isBeanMatched checks whether a bean matches the given type and name selector.
func isBeanMatched(t reflect.Type, s string, b *gs_bean.BeanDefinition) bool {
	if s != "" && s != b.GetName() {
		return false
	}
	if t != nil && t != b.GetType() {
		if !slices.Contains(b.GetExports(), t) {
			return false
		}
	}
	return true
}

// resolveBeans evaluates all beans in the container against their conditions.
// Each bean's status is updated: StatusResolved if all conditions pass,
// or StatusDeleted if any condition fails.
func (c *Resolving) resolveBeans(p flatten.Storage) error {
	ctx := &ConditionContext{p: p, c: c}
	for _, b := range c.beans {
		if err := ctx.resolveBean(b); err != nil {
			return errutil.Explain(err, "failed to resolve bean [%s]", b)
		}
	}
	return nil
}

// ConditionContext represents the context for evaluating bean conditions.
type ConditionContext struct {
	c *Resolving
	p flatten.Storage
}

// resolveBean evaluates all conditions of the given bean within this context.
// - If the bean is already resolving or resolved, it is skipped.
// - If any condition fails, the bean's status is set to StatusDeleted.
// - If all conditions pass, the status is set to StatusResolved.
func (c *ConditionContext) resolveBean(b *gs_bean.BeanDefinition) error {
	if b.GetStatus() >= gs_bean.StatusResolving {
		return nil
	}
	b.SetStatus(gs_bean.StatusResolving)
	for _, cond := range b.Conditions() {
		if ok, err := cond.Matches(c); err != nil {
			return err
		} else if !ok {
			b.SetStatus(gs_bean.StatusDeleted)
			return nil
		}
	}
	b.SetStatus(gs_bean.StatusResolved)
	return nil
}

// Has returns true if the given configuration key exists in the storage.
func (c *ConditionContext) Has(key string) bool {
	return c.p.Exists(key)
}

// Prop returns the string value of the given configuration key.
// Returns (value, true) if the key exists, ("", false) otherwise.
func (c *ConditionContext) Prop(key string) (string, bool) {
	return c.p.Value(key)
}

// Find searches for all active beans matching the given BeanID (type and/or name).
// - Skips beans that are resolving or deleted.
// - Calls resolveBean to ensure each matching bean still satisfies its conditions.
// Returns a slice of ConditionBean and an error if any resolution fails.
func (c *ConditionContext) Find(beanID gs.BeanID) ([]gs.ConditionBean, error) {
	var found []gs.ConditionBean
	for _, b := range c.c.beans {
		if b.GetStatus() == gs_bean.StatusResolving || b.GetStatus() == gs_bean.StatusDeleted {
			continue
		}
		if !isBeanMatched(beanID.Type, beanID.Name, b) {
			continue
		}
		if err := c.resolveBean(b); err != nil {
			return nil, err
		}
		if b.GetStatus() == gs_bean.StatusDeleted {
			continue
		}
		found = append(found, b)
	}
	return found, nil
}

// checkDuplicateBeans ensures that no two beans share the same type and name.
func (c *Resolving) checkDuplicateBeans() error {
	beansByID := make(map[gs.BeanID]*gs_bean.BeanDefinition)
	for _, b := range c.beans {
		if b.GetStatus() == gs_bean.StatusDeleted {
			continue
		}
		for _, t := range append(b.GetExports(), b.GetType()) {
			beanID := gs.BeanID{Name: b.GetName(), Type: t}
			if d, ok := beansByID[beanID]; ok {
				return errutil.Explain(nil, "found duplicate beans [%s] [%s]", b, d)
			}
			beansByID[beanID] = b
		}
	}
	return nil
}
