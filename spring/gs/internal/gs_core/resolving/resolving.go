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
	"context"
	"reflect"
	"regexp"
	"slices"

	"go-spring.org/log"
	"go-spring.org/spring/gs/internal/gs"
	"go-spring.org/spring/gs/internal/gs_bean"
	"go-spring.org/spring/gs/internal/gs_cond"
	"go-spring.org/spring/gs/internal/gs_init"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/funcutil"
)

// Resolving is the core container managing BeanDefinitions.
// It supports registering beans, applying modules, scanning configuration beans,
// resolving conditional beans, and checking for duplicates.
type Resolving struct {
	state gs.RefreshState           // current refresh state, see gs.RefreshState
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
// objOrCtor may be an existing instance or a constructor function. It panics if
// the container is already Refreshing or Refreshed. The returned BeanDefinition
// has caller information populated.
func (c *Resolving) Provide(objOrCtor any, args ...gs.Arg) *gs_bean.BeanDefinition {
	if c.state >= gs.Refreshing {
		panic("container is already refreshing or refreshed")
	}
	b := gs_bean.NewBean(objOrCtor, args...)
	c.beans = append(c.beans, b)
	b = b.Caller(2)
	log.Debug(context.Background(), log.TagAppDef, func() []log.Field {
		return []log.Field{log.String("bean", b.String()), log.Msg("container bean provided")}
	})
	return b
}

// Refresh performs the full container initialization lifecycle.
// It merges beans, applies modules, scans configurations, resolves conditions,
// and checks for duplicates. Returns an error if the container is not in the
// default state or if any step fails.
func (c *Resolving) Refresh(p flatten.Storage) error {
	if c.state != gs.RefreshDefault {
		return errutil.Explain(nil, "container is already refreshing or refreshed")
	}
	c.state = gs.RefreshPrepare

	globalBeans := gs_init.Beans()
	globalModules := gs_init.Modules()
	log.Debug(context.Background(), log.TagAppDef, func() []log.Field {
		return []log.Field{
			log.Int("container_beans", len(c.beans)),
			log.Int("global_beans", len(globalBeans)),
			log.Int("modules", len(globalModules)),
			log.Msg("resolving phase: merging container beans, global beans and modules"),
		}
	})

	c.beans = append(globalBeans, c.beans...)
	if err := c.applyModules(p); err != nil {
		return errutil.Explain(err, "apply modules failed")
	}

	c.state = gs.Refreshing
	log.Debug(context.Background(), log.TagAppDef, func() []log.Field {
		return []log.Field{log.Int("beans", len(c.beans)), log.Msg("resolving phase: beans after module application")}
	})

	if err := c.scanConfigurations(); err != nil {
		return errutil.Explain(err, "scan configurations failed")
	}
	log.Debug(context.Background(), log.TagAppDef, func() []log.Field {
		return []log.Field{log.Int("beans", len(c.beans)), log.Msg("resolving phase: beans after config scanning")}
	})

	if err := c.resolveBeans(p); err != nil {
		return errutil.Explain(err, "resolve beans failed")
	}

	if err := c.checkDuplicateBeans(); err != nil {
		return errutil.Explain(err, "check duplicate beans failed")
	}

	active := len(c.Beans())
	c.state = gs.Refreshed
	log.Debug(context.Background(), log.TagAppDef, func() []log.Field {
		return []log.Field{log.Int("active_beans", active), log.Msg("resolving phase complete")}
	})
	return nil
}

// applyModules iterates over all globally registered modules and executes
// those without a condition or whose conditions match the given context.
func (c *Resolving) applyModules(p flatten.Storage) error {
	ctx := &ConditionContext{props: p, r: c}
	for _, m := range gs_init.Modules() {
		if m.Condition != nil {
			if ok, err := m.Condition.Matches(ctx); err != nil {
				return errutil.Explain(err, "failed to apply module at %s", m.FileLine)
			} else if !ok {
				log.Debug(context.Background(), log.TagAppDef, func() []log.Field {
					return []log.Field{log.String("module", m.FileLine), log.Msg("module skipped (condition not met)")}
				})
				continue
			}
		}
		if err := m.ModuleFunc(c, p); err != nil {
			return errutil.Explain(err, "failed to apply module at %s", m.FileLine)
		}
		log.Debug(context.Background(), log.TagAppDef, func() []log.Field {
			return []log.Field{log.String("module", m.FileLine), log.Msg("module applied")}
		})
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
			return errutil.Explain(err, "failed to scan configuration bean %s", b)
		}
		c.beans = append(c.beans, beans...)
		log.Debug(context.Background(), log.TagAppDef, func() []log.Field {
			return []log.Field{log.String("bean", b.String()), log.Int("children", len(beans)), log.Msg("config bean scanned")}
		})
	}
	return nil
}

// scanConfiguration scans the methods of a configuration bean (bd) and
// registers methods as new beans according to include/exclude regex patterns.
//   - If Includes is empty, defaults to methods matching "New.*".
//   - Methods matching any Exclude pattern are skipped.
//   - Each registered bean gets a name "<ConfigBeanName>_<MethodName>"
//     and a condition OnBeanID of the configuration bean.
func (c *Resolving) scanConfiguration(bd *gs_bean.BeanDefinition) ([]*gs_bean.BeanDefinition, error) {
	var (
		includes []*regexp.Regexp
		excludes []*regexp.Regexp
	)

	param := bd.GetConfiguration()

	patterns := param.Includes
	if len(patterns) == 0 {
		patterns = []string{"New.*"}
	}
	for _, s := range patterns {
		rx, err := regexp.Compile(s)
		if err != nil {
			return nil, errutil.Explain(err, "invalid regexp '%s'", s)
		}
		includes = append(includes, rx)
	}

	patterns = param.Excludes
	for _, s := range patterns {
		rx, err := regexp.Compile(s)
		if err != nil {
			return nil, errutil.Explain(err, "invalid regexp '%s'", s)
		}
		excludes = append(excludes, rx)
	}

	var children []*gs_bean.BeanDefinition
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
			children = append(children, b)
			log.Trace(context.Background(), log.TagAppDef, func() []log.Field {
				return []log.Field{log.String("method", m.Name), log.String("bean", b.String()), log.Msg("config method registered")}
			})
			break
		}
	}
	return children, nil
}

// isBeanMatched checks whether a bean matches the given type and name selector.
func isBeanMatched(beanType reflect.Type, name string, b *gs_bean.BeanDefinition) bool {
	if name != "" && name != b.GetName() {
		return false
	}
	if beanType != nil && beanType != b.GetType() {
		if !slices.Contains(b.GetExports(), beanType) {
			return false
		}
	}
	return true
}

// resolveBeans evaluates all beans in the container against their conditions.
// Each bean's status is updated: StatusResolved if all conditions pass,
// or StatusDeleted if any condition fails.
func (c *Resolving) resolveBeans(p flatten.Storage) error {
	ctx := &ConditionContext{props: p, r: c}
	for _, b := range c.beans {
		if err := ctx.resolveBean(b); err != nil {
			return errutil.Explain(err, "failed to resolve bean %s", b)
		}
	}
	return nil
}

// ConditionContext represents the context for evaluating bean conditions.
type ConditionContext struct {
	r     *Resolving
	props flatten.Storage
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
		ok, err := cond.Matches(c)
		if err != nil {
			return errutil.Explain(err, "condition matches failed for bean %s", b)
		}
		log.Trace(context.Background(), log.TagAppDef, func() []log.Field {
			return []log.Field{log.String("bean", b.String()), log.Any("condition", cond), log.Bool("matched", ok), log.Msg("condition check")}
		})
		if !ok {
			b.SetStatus(gs_bean.StatusDeleted)
			log.Debug(context.Background(), log.TagAppDef, func() []log.Field {
				return []log.Field{log.String("bean", b.String()), log.Any("condition", cond), log.Msg("bean resolved (deleted, condition failed)")}
			})
			return nil
		}
	}
	b.SetStatus(gs_bean.StatusResolved)
	log.Debug(context.Background(), log.TagAppDef, func() []log.Field {
		return []log.Field{log.String("bean", b.String()), log.Msg("bean resolved (active)")}
	})
	return nil
}

// Has returns true if the given configuration key exists in the storage.
func (c *ConditionContext) Has(key string) bool {
	return c.props.Exists(key)
}

// Prop returns the string value of the given configuration key.
// Returns (value, true) if the key exists, ("", false) otherwise.
func (c *ConditionContext) Prop(key string) (string, bool) {
	return c.props.Value(key)
}

// Find searches for all active beans matching the given BeanID (type and/or name).
// - Skips beans that are resolving or deleted.
// - Calls resolveBean to ensure each matching bean still satisfies its conditions.
// Returns a slice of ConditionBean and an error if any resolution fails.
func (c *ConditionContext) Find(beanID gs.BeanID) ([]gs.ConditionBean, error) {
	var found []gs.ConditionBean
	for _, b := range c.r.beans {
		if b.GetStatus() == gs_bean.StatusResolving || b.GetStatus() == gs_bean.StatusDeleted {
			continue
		}
		if !isBeanMatched(beanID.Type, beanID.Name, b) {
			continue
		}
		if err := c.resolveBean(b); err != nil {
			return nil, errutil.Explain(err, "find bean by BeanID=%s failed", beanID)
		}
		if b.GetStatus() == gs_bean.StatusDeleted {
			continue
		}
		found = append(found, b)
	}
	log.Trace(context.Background(), log.TagAppDef, func() []log.Field {
		return []log.Field{log.Any("bean_id", beanID), log.Int("found", len(found)), log.Msg("find beans")}
	})
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
				log.Debug(context.Background(), log.TagAppDef, func() []log.Field {
					return []log.Field{log.String("bean", b.String()), log.String("conflict", d.String()), log.Any("type", t), log.Msg("duplicate bean detected")}
				})
				return errutil.Explain(nil, "found duplicate beans %s and %s", b, d)
			}
			beansByID[beanID] = b
		}
	}
	log.Debugf(context.Background(), log.TagAppDef, "no duplicate beans detected")
	return nil
}
