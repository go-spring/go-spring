/*
 * Copyright 2024 The Go-Spring Authors.
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

package injecting

import (
	"bytes"
	"container/list"
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"

	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs/internal/gs"
	"go-spring.org/spring/gs/internal/gs_bean"
	"go-spring.org/spring/gs/internal/gs_dync"
	"go-spring.org/spring/gs/internal/gs_util"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/listutil"
	"go-spring.org/stdlib/patchutil"
	"go-spring.org/stdlib/typeutil"
)

// refreshState represents the state of a refresh operation.
type refreshState int

const (
	RefreshDefault = refreshState(iota) // Not refreshed yet
	Refreshing                          // Currently refreshing
	Refreshed                           // Successfully refreshed
)

// Injecting is the core IoC container component.
// It handles bean creation, dependency injection, lifecycle management,
// dynamic property updates, and destroy callbacks.
type Injecting struct {
	// Dynamic properties provider
	p *gs_dync.Properties
	// Cleanup functions in reverse order
	destroyers []func()
}

// New creates a new Injecting instance.
func New(p flatten.Storage) *Injecting {
	return &Injecting{
		p: gs_dync.New(p),
	}
}

// DynamicObjectsCount returns the number of objects that can be dynamically refreshed.
func (c *Injecting) DynamicObjectsCount() int {
	if c.p == nil {
		return 0
	}
	return c.p.ObjectsCount()
}

// RefreshProperties updates the dynamic properties in the container.
func (c *Injecting) RefreshProperties(p flatten.Storage) error {
	return c.p.Refresh(p)
}

// Refresh wires all provided beans and prepares them for use.
// It performs the following operations:
//
//  1. Builds indexes for bean lookup by name and type (by name and by type).
//  2. Wires all root beans (entry points of the dependency graph), recursively wiring dependencies.
//  3. Handles fields tagged with ',lazy' for deferred injection.
//     Note: lazy wiring only applies to explicitly marked fields and does not
//     resolve arbitrary circular dependencies.
//  4. Registers destroyer callbacks for beans in dependency-safe order.
//  5. Cleans up metadata.
//
// Behavior is influenced by properties:
// - spring.allow-circular-references: whether lazy circular references are allowed.
// - spring.force-autowire-is-nullable: whether missing dependencies are treated as nullable.
func (c *Injecting) Refresh(roots, beans []*gs_bean.BeanDefinition) (err error) {
	var forceAutowireIsNullable bool
	{
		s, _ := c.p.Data().Value("spring.force-autowire-is-nullable")
		forceAutowireIsNullable, _ = strconv.ParseBool(s)
	}

	// Index beans by name and type for lookup
	beansByName := make(map[string][]*gs_bean.BeanDefinition)
	beansByType := make(map[reflect.Type][]*gs_bean.BeanDefinition)
	for _, b := range beans {
		beansByName[b.GetName()] = append(beansByName[b.GetName()], b)
		beansByType[b.GetType()] = append(beansByType[b.GetType()], b)
		for _, t := range b.GetExports() { // Register additional exported types
			beansByType[t] = append(beansByType[t], b)
		}
	}

	stack := NewStack()
	defer func() {
		// If an error occurred, or there are unresolved beans in the stack,
		// enrich the error message with the dependency path for easier debugging.
		if err != nil || len(stack.beans) > 0 {
			log.Errorf(context.Background(), log.TagAppDef, "%s", err)
		}
	}()

	r := &Injector{
		state:                   RefreshDefault,
		p:                       c.p,
		beansByName:             beansByName,
		beansByType:             beansByType,
		forceAutowireIsNullable: forceAutowireIsNullable,
	}

	// Step 1: Wire all root beans.
	r.state = Refreshing
	for _, b := range roots {
		if err = r.wireBean(b, stack); err != nil {
			return err
		}
	}
	r.state = Refreshed

	// Step 2: Handle lazy fields caused by circular dependencies.
	for _, f := range stack.lazyFields {
		tag := strings.TrimSuffix(f.tag, ",lazy")
		if err = r.autowire(f.v, tag, stack); err != nil {
			return err
		}
	}

	// Step 3: Collect destroyer callbacks in dependency-safe order.
	c.destroyers = stack.getSortedDestroyers()

	// Step 4: Clean up metadata.
	if c.p.ObjectsCount() == 0 {
		c.p = nil
	}
	return nil
}

// Close shuts down the container by invoking all registered destroyer callbacks.
// The destroyers are executed in reverse order respecting dependency relationships,
// ensuring that beans are destroyed after the beans they depend on.
// Any errors returned from destroy methods are logged but do not stop the shutdown process.
func (c *Injecting) Close() {
	for _, f := range c.destroyers {
		f()
	}
}

// Injector performs core dependency injection and bean lifecycle management.
// Responsibilities include:
// - Constructor invocation and creation of bean values.
// - Field injection and wiring of struct dependencies.
// - Initialization callbacks execution.
// - Bean status management (creating, created, wired).
// - Lazy field handling and circular dependency detection.
// - Respecting forceAutowireIsNullable flag to treat missing dependencies as optional.
type Injector struct {
	state                   refreshState                               // Current wiring state
	p                       *gs_dync.Properties                        // Property resolver
	beansByName             map[string][]*gs_bean.BeanDefinition       // Beans indexed by name
	beansByType             map[reflect.Type][]*gs_bean.BeanDefinition // Beans indexed by type
	forceAutowireIsNullable bool                                       // Treat missing references as nullable
}

// findBeans retrieves all beans matching the specified BeanID.
// Matching is done first by type (if Type is not nil), then filtered by Name (if Name is not empty).
// Returns a slice of BeanDefinition; may be empty if no match is found.
func (c *Injector) findBeans(beanID gs.BeanID) []*gs_bean.BeanDefinition {
	var beans []*gs_bean.BeanDefinition
	if beanID.Type != nil {
		beans = c.beansByType[beanID.Type]
	}
	if beanID.Name != "" {
		var ret []*gs_bean.BeanDefinition
		for _, b := range beans {
			if beanID.Name == b.GetName() {
				ret = append(ret, b)
			}
		}
		beans = ret
	}
	return beans
}

// WireTag represents the parsed structure of an injection tag.
// Format: "BeanName?" where "?" marks the dependency as nullable.
type WireTag struct {
	beanName string // The target bean's name
	nullable bool   // Whether the injection can be nil
}

// String converts a WireTag back to its string representation.
func (tag WireTag) String() string {
	var sb strings.Builder
	sb.WriteString(tag.beanName)
	if tag.nullable {
		sb.WriteString("?")
	}
	return sb.String()
}

// parseWireTag parses a raw wire tag string into a structured WireTag.
func parseWireTag(str string) (tag WireTag) {
	if str != "" {
		if n := len(str) - 1; str[n] == '?' {
			tag.beanName = str[:n]
			tag.nullable = true
		} else {
			tag.beanName = str
		}
	}
	return
}

// toWireString converts a slice of WireTags into a comma-separated string.
func toWireString(tags []WireTag) string {
	var buf bytes.Buffer
	for i, tag := range tags {
		buf.WriteString(tag.String())
		if i < len(tags)-1 {
			buf.WriteByte(',')
		}
	}
	return buf.String()
}

// getBean retrieves a single bean of the given type that matches the WireTag.
// Behavior:
// - Validates the type is suitable for injection.
// - Filters by bean name if WireTag.beanName is set.
// - Respects WireTag.nullable; returns nil if no matching bean and nullable.
// - Returns an error if multiple matching beans are found.
// - If the container is currently Refreshing, the bean will be wired before returning.
func (c *Injector) getBean(t reflect.Type, tag WireTag, stack *Stack) (*gs_bean.BeanDefinition, error) {
	// Ensure the target type is valid for injection.
	if !typeutil.IsBeanInjectionTarget(t) {
		return nil, errutil.Explain(nil, "%s is not a valid injection target type", t.String())
	}

	var foundBeans []*gs_bean.BeanDefinition
	for _, b := range c.beansByType[t] {
		if tag.beanName == "" || tag.beanName == b.GetName() {
			foundBeans = append(foundBeans, b)
		}
	}

	if len(foundBeans) == 0 {
		if tag.nullable {
			return nil, nil
		}
		return nil, errutil.Explain(nil, "cannot find bean: %q type: %q", tag, t)
	}

	if len(foundBeans) > 1 {
		msg := fmt.Sprintf("found %d beans, bean:%q type:%q [", len(foundBeans), tag, t)
		for _, b := range foundBeans {
			msg += "( " + b.String() + " ), "
		}
		msg = msg[:len(msg)-2] + "]"
		return nil, errutil.Explain(nil, "%s", msg)
	}

	b := foundBeans[0]
	if c.state == Refreshing {
		if err := c.wireBean(b, stack); err != nil {
			return nil, err
		}
	}
	return b, nil
}

// getBeans retrieves a collection (slice or map) of beans matching the element type.
// Supports optional WireTags for ordering and selection.
// - Tags with '*' act as a wildcard for remaining unordered beans.
// - Tags marked nullable are skipped if not found.
// - Ensures deterministic order: before '*' -> '*' -> after '*'.
// - Returns an error if required beans are missing or duplicates are detected.
// - During Refreshing state, each returned bean is wired before inclusion.
func (c *Injector) getBeans(t reflect.Type, tags []WireTag, nullable bool,
	stack *Stack) ([]*gs_bean.BeanDefinition, error) {

	et := t.Elem()
	if !typeutil.IsBeanInjectionTarget(et) {
		return nil, errutil.Explain(nil, "%s is not a valid injection target type", t.String())
	}

	beans := c.beansByType[et]

	// Process bean tags to filter and order beans
	if len(tags) > 0 {
		var (
			anyBeans  []*gs_bean.BeanDefinition // beans to be placed in the '*' section
			afterAny  []int                     // beans to appear after the '*'
			beforeAny []int                     // beans to appear before the '*'
		)
		foundAny := false
		for _, item := range tags {

			// If we see the "*" wildcard, record its presence
			if item.beanName == "*" {
				if foundAny {
					return nil, errutil.Explain(nil, "more than one * in collection %q", tags)
				}
				foundAny = true
				continue
			}

			// Find beans with the specified name
			var founds []int
			for i, b := range beans {
				if item.beanName == b.GetName() {
					founds = append(founds, i)
				}
			}

			// Error if there are multiple beans with the same name
			if len(founds) > 1 {
				msg := fmt.Sprintf("found %d beans, bean:%q type:%q [", len(founds), item, t)
				for _, i := range founds {
					msg += "( " + beans[i].String() + " ), "
				}
				msg = msg[:len(msg)-2] + "]"
				return nil, errutil.Explain(nil, "%s", msg)
			}

			// Error if no matching bean is found (unless the tag is nullable)
			if len(founds) == 0 {
				if item.nullable {
					continue
				}
				return nil, errutil.Explain(nil, "cannot find bean: %q type: %q", item, t)
			}

			// Classify beans as before or after the '*'
			if foundAny {
				afterAny = append(afterAny, founds[0])
			} else {
				beforeAny = append(beforeAny, founds[0])
			}
		}

		// For the '*' wildcard, include all other beans that were not explicitly listed
		if foundAny {
			temp := append(beforeAny, afterAny...)
			for i := range len(beans) {
				if slices.Contains(temp, i) {
					continue
				}
				anyBeans = append(anyBeans, beans[i])
			}
		}

		// Assemble beans in the correct order: beforeAny -> anyBeans -> afterAny
		n := len(beforeAny) + len(anyBeans) + len(afterAny)
		arr := make([]*gs_bean.BeanDefinition, 0, n)
		for _, i := range beforeAny {
			arr = append(arr, beans[i])
		}
		sort.SliceStable(anyBeans, func(i, j int) bool {
			return anyBeans[i].GetName() < anyBeans[j].GetName()
		})
		for _, b := range anyBeans {
			arr = append(arr, b)
		}
		for _, i := range afterAny {
			arr = append(arr, beans[i])
		}
		beans = arr

	} else {
		arr := make([]*gs_bean.BeanDefinition, 0, len(beans))
		for _, b := range beans {
			arr = append(arr, b)
		}
		sort.SliceStable(arr, func(i, j int) bool {
			return arr[i].GetName() < arr[j].GetName()
		})
		beans = arr
	}

	// Handle the case where no beans were found
	if len(beans) == 0 {
		if nullable {
			return nil, nil
		}
		return nil, errutil.Explain(nil, "no beans collected for %q", toWireString(tags))
	}

	seen := make(map[*gs_bean.BeanDefinition]struct{}, len(beans))
	for _, b := range beans {
		if _, ok := seen[b]; ok {
			return nil, errutil.Explain(nil, "duplicate bean %s in collection %q", b.String(), toWireString(tags))
		}
		seen[b] = struct{}{}
	}

	// If the container is in the refreshing state, wire the beans before returning them
	if c.state == Refreshing {
		for _, b := range beans {
			if err := c.wireBean(b, stack); err != nil {
				return nil, err
			}
		}
	}
	return beans, nil
}

// autowire injects dependencies into the given reflect.Value according to the tag string.
// - Supports single beans, slices, and maps.
// - Resolves placeholders (e.g., ${...}) from configuration.
// - Honors lazy or nullable tags, including forceAutowireIsNullable setting.
// - Populates slices/maps with wired bean values, sorting slices by bean name.
func (c *Injector) autowire(v reflect.Value, str string, stack *Stack) error {
	// Resolve placeholder expressions (e.g., ${...}) from configuration
	str, err := conf.Resolve(c.p.Data(), str)
	if err != nil {
		return err
	}

	switch v.Kind() {
	case reflect.Array: // do nothing
		return nil
	case reflect.Map, reflect.Slice:
		{
			// Handle collection types
			var nullable bool
			var tags []WireTag

			// Parse the tag string to determine nullability and tag list
			if str != "" {
				nullable = true
				if str != "?" {
					for s := range strings.SplitSeq(str, ",") {
						g := parseWireTag(s)
						tags = append(tags, g)
						if !g.nullable {
							nullable = false
						}
					}
				}
			}

			// If forced nullable mode is enabled, override all tags
			if c.forceAutowireIsNullable {
				for i := range len(tags) {
					tags[i].nullable = true
				}
				nullable = true
			}

			// Retrieve the beans matching the tag and type
			beans, err := c.getBeans(v.Type(), tags, nullable, stack)
			if err != nil {
				return err
			}

			// Populate the collection field with the resolved beans
			switch v.Kind() {
			case reflect.Slice:
				ret := reflect.MakeSlice(v.Type(), 0, 0)
				for _, b := range beans {
					ret = reflect.Append(ret, b.GetValue())
				}
				v.Set(ret)
			case reflect.Map:
				// Non-string map keys are an uncommon invalid injection target; keeping the
				// check here avoids adding noise to the common collection lookup path.
				if v.Type().Key().Kind() != reflect.String {
					return errutil.Explain(nil, "map key should be string")
				}
				ret := reflect.MakeMap(v.Type())
				for _, b := range beans {
					ret.SetMapIndex(reflect.ValueOf(b.GetName()), b.GetValue())
				}
				v.Set(ret)
			default: // for linter
			}
			return nil
		}
	default:
		// Handle single bean injection
		g := parseWireTag(str)
		if c.forceAutowireIsNullable {
			g.nullable = true
		}
		b, err := c.getBean(v.Type(), g, stack)
		if err != nil {
			return err
		}
		if b != nil {
			v.Set(b.GetValue())
		}
		return nil
	}
}

// wireBean fully constructs, injects dependencies, and initializes the given BeanDefinition.
// Steps:
// 1. Wires beans declared in GetDependsOn() recursively.
// 2. Pushes bean onto the wiring stack (registers destroyer if it has one).
// 3. Detects circular dependencies; returns error if detected.
// 4. Invokes the bean's constructor (if any) via getBeanValue.
// 5. Performs field-level wiring using wireBeanValue.
// 6. Calls the bean's Init callback if defined.
// After completion, bean status is set to StatusWired and popped from the stack.
func (c *Injector) wireBean(b *gs_bean.BeanDefinition, stack *Stack) error {
	//fmt.Println(b.String())

	// Wire all dependent beans before creating the current bean
	for _, s := range b.GetDependsOn() {
		for _, d := range c.findBeans(s) {
			if err := c.wireBean(d, stack); err != nil {
				return err
			}
		}
	}

	stack.pushBean(b)

	// If the bean is already being created (StatusCreating), we have a circular dependency
	// because it's already in the current call stack and being re-entered recursively.
	// Circular dependencies cannot be resolved without lazy injection.
	if b.GetStatus() == gs_bean.StatusCreating {
		stack.popBean()
		return errutil.Explain(nil, "circular autowire dependency detected")
	}

	// If the bean is already created or wired (StatusCreated or StatusWired), return early
	// pop to keep the stack balanced since we just pushed it.
	if b.GetStatus() >= gs_bean.StatusCreating {
		stack.popBean()
		return nil
	}

	// Mark the bean as currently being created
	b.SetStatus(gs_bean.StatusCreating)

	// Retrieve the actual value for the bean (e.g., via its factory method)
	v, err := c.getBeanValue(b, stack)
	if err != nil {
		return err
	}

	b.SetStatus(gs_bean.StatusCreated)

	// If the bean is valid, inject its internal dependencies
	if v.IsValid() {

		// Perform field-level wiring on the bean value
		if err = c.wireBeanValue(v, v.Type(), stack); err != nil {
			return err
		}

		// Invoke the bean's initialization method if defined
		if b.GetInit() != nil {
			fnValue := reflect.ValueOf(b.GetInit())
			out := fnValue.Call([]reflect.Value{b.GetValue()})
			if len(out) > 0 && !out[0].IsNil() {
				return out[0].Interface().(error)
			}
		}
	}

	// Mark the bean as fully wired and remove it from the stack
	b.SetStatus(gs_bean.StatusWired)
	stack.popBean()
	return nil
}

// getBeanValue invokes the constructor (if present) of a bean and handles return values and errors.
func (c *Injector) getBeanValue(b *gs_bean.BeanDefinition, stack *Stack) (reflect.Value, error) {

	// If there is no constructor, return the pre-existing value
	if b.Callable() == nil {
		return b.GetValue(), nil
	}

	// Invoke the constructor
	out, err := b.Callable().Call(NewArgContext(c, stack))
	if err != nil {
		if c.forceAutowireIsNullable {
			log.Warnf(context.Background(), log.TagAppDef, "autowire error: %v", err)
			return reflect.Value{}, nil
		}
		return reflect.Value{}, err
	}

	// Check if the last return value is an error
	if o := out[len(out)-1]; typeutil.IsErrorType(o.Type()) {
		if err, ok := o.Interface().(error); ok && err != nil {
			if c.forceAutowireIsNullable {
				log.Warnf(context.Background(), log.TagAppDef, "autowire error: %v", err)
				return reflect.Value{}, nil
			}
			return reflect.Value{}, err
		}
	}

	// Assign the returned value to the bean
	if val := out[0]; typeutil.IsBeanType(val.Type()) {
		// Convert interface values to pointers if necessary
		if !val.IsNil() && val.Kind() == reflect.Interface && typeutil.IsPropBindingTarget(val.Elem().Type()) {
			v := reflect.New(val.Elem().Type())
			v.Elem().Set(val.Elem())
			b.GetValue().Set(v)
		} else {
			b.GetValue().Set(val)
		}
	} else {
		b.GetValue().Elem().Set(val)
	}

	// Ensure the value is not nil
	if b.GetValue().IsNil() {
		return reflect.Value{}, errutil.Explain(nil, "%s returned nil", b.String())
	}

	// If the value is an interface, unwrap it
	v := b.GetValue()
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	return v, nil
}

// wireBeanValue injects dependencies into all struct fields of the given bean value.
func (c *Injector) wireBeanValue(v reflect.Value, t reflect.Type, stack *Stack) error {

	// Dereference pointers to obtain the underlying struct
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
		t = t.Elem()
	}

	// If it's not a struct, nothing to wire
	if v.Kind() != reflect.Struct {
		return nil
	}

	// Use the type name for binding paths
	typeName := t.Name()
	if typeName == "" {
		typeName = t.String()
	}

	param := conf.BindParam{Path: typeName}
	return c.wireStruct(v, t, param, stack)
}

// InjectionError wraps an injection failure at a specific field path.
type InjectionError struct {
	path string // Field path where injection failed
	err  error  // The underlying error that caused the failure
}

// Path returns the full field path where injection failed.
func (e *InjectionError) Path() string {
	return e.path
}

// Unwrap returns the root error.
func (e *InjectionError) Unwrap() error {
	return e.err
}

// Error formats the error with the field path and the root error.
func (e *InjectionError) Error() string {
	return fmt.Sprintf("injection failed at %s: %v", e.path, e.err)
}

// wireStruct inspects each field of a struct and performs wiring as needed.
// - Handles 'autowire' and 'inject' tags for dependency injection.
// - Handles 'value' tags for configuration binding.
// - Supports anonymous/embedded structs recursively.
// - Defers lazy injection fields by appending to the stack's lazyFields.
func (c *Injector) wireStruct(v reflect.Value, t reflect.Type, opt conf.BindParam, stack *Stack) error {
	for i := range t.NumField() {
		ft := t.Field(i)
		fv := v.Field(i)

		// Patch unexported fields so they can be set via reflection
		if !fv.CanInterface() {
			fv = patchutil.PatchValue(fv)
		}

		fieldPath := opt.Path + "." + ft.Name

		// Look for "autowire" or "inject" tags
		tag, ok := ft.Tag.Lookup("autowire")
		if !ok {
			tag, ok = ft.Tag.Lookup("inject")
		}
		if ok {
			// Handle lazy-injected fields
			if strings.HasSuffix(tag, ",lazy") {
				f := LazyField{v: fv, path: fieldPath, tag: tag}
				stack.lazyFields = append(stack.lazyFields, f)
			} else {
				if err := c.autowire(fv, tag, stack); err != nil {
					if _, ok = errors.AsType[*InjectionError](err); ok {
						return err
					}
					return &InjectionError{path: fieldPath, err: err}
				}
			}
			continue
		}

		subParam := conf.BindParam{
			Key:  opt.Key,
			Path: fieldPath,
		}

		// If the field has a "value" tag, bind configuration to it
		if tag, ok = ft.Tag.Lookup("value"); ok {
			if err := subParam.BindTag(tag, ft.Tag); err != nil {
				return err
			}
			if ft.Anonymous && ft.Type.Kind() == reflect.Struct {
				// Recursively process embedded structs
				if err := c.wireStruct(fv, ft.Type, subParam, stack); err != nil {
					return err
				}
			} else {
				// Refresh the field value from configuration
				if err := c.p.RefreshField(fv.Addr(), subParam); err != nil {
					return err
				}
			}
			continue
		}

		// Recursively process anonymous struct fields
		if ft.Anonymous && ft.Type.Kind() == reflect.Struct {
			if err := c.wireStruct(fv, ft.Type, subParam, stack); err != nil {
				return err
			}
		}
	}
	return nil
}

// destroyer represents a bean's cleanup (destroy) function
// and the dependencies that must be destroyed before it.
type destroyer struct {
	current *gs_bean.BeanDefinition   // Bean that provides this destroyer
	depends []*gs_bean.BeanDefinition // Beans that must be destroyed before current
}

// isDependOn reports whether this destroyer depends on the given bean.
func (d *destroyer) isDependOn(b *gs_bean.BeanDefinition) bool {
	return slices.Contains(d.depends, b)
}

// dependOn adds the given bean to this destroyer's dependency list
// if it is not already present.
func (d *destroyer) dependOn(b *gs_bean.BeanDefinition) {
	if d.isDependOn(b) {
		return
	}
	d.depends = append(d.depends, b)
}

// LazyField represents a field in a struct that should be injected lazily.
type LazyField struct {
	v    reflect.Value // The field value that will be injected later
	path string        // Hierarchical path of the field
	tag  string        // Original tag (e.g. "autowire") for this field
}

type destroyerList = listutil.List[*gs_bean.BeanDefinition]

// Stack represents the runtime context during bean wiring.
// It keeps track of the current wiring call stack, lazily injected fields,
// and the ordering of destroyers for proper shutdown.
type Stack struct {
	beans        []*gs_bean.BeanDefinition // The stack of beans currently being wired
	lazyFields   []LazyField               // Fields deferred due to lazy injection
	destroyers   *destroyerList            // Ordered list of destroyers
	destroyerMap map[gs.BeanID]*destroyer  // Fast lookup map for destroyers by bean ID
}

// NewStack creates and initializes a new Stack for a fresh Refresh or Wire operation.
func NewStack() *Stack {
	return &Stack{
		destroyers:   listutil.New[*gs_bean.BeanDefinition](),
		destroyerMap: make(map[gs.BeanID]*destroyer),
	}
}

// pushBean pushes a bean onto the wiring stack.
// Used to keep track of current wiring path for cycle detection.
func (s *Stack) pushBean(b *gs_bean.BeanDefinition) {
	log.Debugf(context.Background(), log.TagAppDef, "push %s %s", b, b.GetStatus())
	s.beans = append(s.beans, b)
	if b.GetDestroy() != nil {
		s.pushDestroyer(b)
	}
}

// popBean pops the most recently added bean from the wiring stack.
func (s *Stack) popBean() {
	n := len(s.beans)
	b := s.beans[n-1]
	if b.GetDestroy() != nil {
		s.popDestroyer()
	}
	s.beans[n-1] = nil // avoid memory leak
	s.beans = s.beans[:n-1]
	log.Debugf(context.Background(), log.TagAppDef, "pop %s %s", b, b.GetStatus())
}

// pushDestroyer registers a destroyer for the given bean.
// It also records dependencies so that beans are destroyed in the correct order.
func (s *Stack) pushDestroyer(b *gs_bean.BeanDefinition) {
	beanID := gs.BeanID{Name: b.GetName(), Type: b.GetType()}

	// Get or create the destroyer entry for this bean
	d, ok := s.destroyerMap[beanID]
	if !ok {
		d = &destroyer{current: b}
		s.destroyerMap[beanID] = d
	}

	// Record dependencies
	for _, depBeanID := range b.GetDependsOn() {
		if depDestroyer, ok := s.destroyerMap[depBeanID]; ok {
			d.dependOn(depDestroyer.current)
		}
	}

	// If there is a previously registered destroyer, current depends on it
	if x := s.destroyers.Back(); x.Valid() {
		s.destroyerMap[x.Value().BeanID()].dependOn(b)
	}

	// Add the current bean to the end of the destroyer list
	s.destroyers.PushBack(b)
}

// popDestroyer removes the last registered destroyer from the ordering list.
func (s *Stack) popDestroyer() {
	s.destroyers.Remove(s.destroyers.Back())
}

// getBeforeDestroyers returns a list of destroyers that the given destroyer depends on.
// This helper is used during topological sorting of destroyers.
func getBeforeDestroyers(destroyers *list.List, i any) *list.List {
	d := i.(*destroyer)
	result := list.New()
	for e := destroyers.Front(); e != nil; e = e.Next() {
		c := e.Value.(*destroyer)
		if d.isDependOn(c.current) {
			result.PushBack(c)
		}
	}
	return result
}

// getSortedDestroyers returns destroyer functions in execution order.
// Topological sort ensures each bean is destroyed after all beans it depends on.
// The returned slice can be safely iterated to close the container.
func (s *Stack) getSortedDestroyers() []func() {

	// Helper to wrap a bean's destroy method as a no-argument function
	destroy := func(v reflect.Value, fn any) func() {
		return func() {
			fnValue := reflect.ValueOf(fn)
			out := fnValue.Call([]reflect.Value{v})
			if len(out) > 0 && !out[0].IsNil() {
				log.Errorf(context.Background(), log.TagAppDef, "%v", out[0].Interface())
			}
		}
	}

	// Copy all destroyers into a new list for sorting
	destroyers := list.New()
	for _, d := range s.destroyerMap {
		destroyers.PushBack(d)
	}

	// Perform a topological sort to respect dependencies
	// (e.g. a bean must be destroyed after the beans it depends on)
	destroyers, _ = gs_util.TopologicalSort(destroyers, getBeforeDestroyers)

	// Convert the sorted destroyers into a slice of executable cleanup functions
	var ret []func()
	for e := destroyers.Front(); e != nil; e = e.Next() {
		d := e.Value.(*destroyer).current
		ret = append(ret, destroy(d.GetValue(), d.GetDestroy()))
	}
	return ret
}

// ArgContext provides runtime context when calling bean factory functions.
// It exposes access to configuration properties, bean lookups, condition checks,
// and allows wiring of parameters during construction.
type ArgContext struct {
	c     *Injector
	stack *Stack
}

// NewArgContext constructs a new ArgContext for a wiring operation.
func NewArgContext(c *Injector, stack *Stack) *ArgContext {
	return &ArgContext{c: c, stack: stack}
}

// Has checks whether a configuration key is present.
func (a *ArgContext) Has(key string) bool {
	return a.c.p.Data().Exists(key)
}

// Prop retrieves a property value, with optional default.
func (a *ArgContext) Prop(key string) (string, bool) {
	return a.c.p.Data().Value(key)
}

// Find retrieves beans matching the given selector.
func (a *ArgContext) Find(beanID gs.BeanID) ([]gs.ConditionBean, error) {
	beans := a.c.findBeans(beanID)
	var ret []gs.ConditionBean
	for _, bean := range beans {
		ret = append(ret, bean)
	}
	return ret, nil
}

// Check evaluates a condition against the current ArgContext.
func (a *ArgContext) Check(c gs.Condition) (bool, error) {
	return c.Matches(a)
}

// Bind binds configuration data into the provided reflect.Value
// based on the given struct tag.
func (a *ArgContext) Bind(v reflect.Value, tag string) error {
	return conf.Bind(a.c.p.Data(), v, tag)
}

// Wire performs dependency injection on the given reflect.Value
// using the specified tag, leveraging the current wiring stack.
func (a *ArgContext) Wire(v reflect.Value, tag string) error {
	return a.c.autowire(v, tag, a.stack)
}
