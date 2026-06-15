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

// Package gs_cond provides a set of composable conditions used to control
// bean registration for Go-Spring framework.
//
// It defines various condition types such as:
//
//   - OnFunc:          Uses a custom function to evaluate a condition.
//   - OnProperty:      Matches based on the presence or value of a property.
//   - OnBean:          Matches if at least one bean exists for a given selector.
//   - OnMissingBean:   Matches if no beans exist for a given selector.
//   - OnSingleBean:    Matches if exactly one bean exists for a given selector.
//   - OnExpression:    Evaluates a custom expression (currently unimplemented).
//   - Not / Or / And / None: Logical combinators for composing multiple conditions.
package gs_cond

import (
	"fmt"
	"strings"

	"go-spring.org/spring/gs/internal/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/funcutil"
)

/********************************* OnFunc ************************************/

// onFunc is an implementation of [gs.Condition] that wraps a user-defined function.
// This allows defining a custom condition that is evaluated at runtime
// based on the provided function.
type onFunc struct {
	fn func(ctx gs.ConditionContext) (bool, error)
}

// OnFunc creates a condition that evaluates using the provided custom function.
func OnFunc(fn func(ctx gs.ConditionContext) (bool, error)) gs.Condition {
	if fn == nil {
		panic("condition function cannot be nil")
	}
	return &onFunc{fn: fn}
}

// Matches executes the wrapped function to determine if the condition is satisfied.
func (c *onFunc) Matches(ctx gs.ConditionContext) (bool, error) {
	ok, err := c.fn(ctx)
	if err != nil {
		return false, errutil.Explain(err, "condition %s matches error", c)
	}
	return ok, nil
}

func (c *onFunc) String() string {
	_, _, fnName := funcutil.FileLine(c.fn)
	return fmt.Sprintf("OnFunc(fn=%s)", fnName)
}

/******************************* OnProperty **********************************/

// PropertyCondition defines a condition that is evaluated based on the value
// of a property in the application context. It provides methods to customize
// behavior for missing properties and specific property values.
type PropertyCondition interface {
	gs.Condition
	MatchIfMissing() PropertyCondition      // Match if the property is missing
	HavingValue(s string) PropertyCondition // Match if the property has a specific value
}

// onProperty implements [PropertyCondition], allowing conditions based on
// the existence and value of properties in the context.
type onProperty struct {
	name           string  // Property name to check
	matchIfMissing bool    // Whether to match when the property is missing
	havingValue    *string // Expected value or expression for comparison
}

// OnProperty creates a new condition that checks for the presence
// and/or value of a specified property.
func OnProperty(name string) PropertyCondition {
	return &onProperty{name: name}
}

// MatchIfMissing sets the condition to match if the property is missing.
func (c *onProperty) MatchIfMissing() PropertyCondition {
	c.matchIfMissing = true
	return c
}

// HavingValue sets the expected value or expression to match.
func (c *onProperty) HavingValue(s string) PropertyCondition {
	c.havingValue = &s
	return c
}

// Matches evaluates the condition based on the property's existence and value.
func (c *onProperty) Matches(ctx gs.ConditionContext) (bool, error) {

	if c.havingValue == nil {
		if ctx.Has(c.name) {
			return true, nil
		}
		return c.matchIfMissing, nil
	}

	val, ok := ctx.Prop(c.name)
	if !ok {
		if ctx.Has(c.name) {
			err := errutil.Explain(nil, "property %s not leaf node", c.name)
			return false, errutil.Explain(err, "condition %s matches error", c)
		}
		return c.matchIfMissing, nil
	}

	havingValue := *c.havingValue
	if !strings.HasPrefix(havingValue, "expr:") {
		return val == havingValue, nil
	}

	// Evaluate as an expression if prefixed with "expr:"
	ok, err := EvalExpr(havingValue[5:], val)
	if err != nil {
		return false, errutil.Explain(err, "condition %s matches error", c)
	}
	return ok, nil
}

func (c *onProperty) String() string {
	var sb strings.Builder
	sb.WriteString("OnProperty(name=")
	sb.WriteString(c.name)
	if c.havingValue != nil {
		sb.WriteString(",havingValue=")
		sb.WriteString(*c.havingValue)
	}
	if c.matchIfMissing {
		sb.WriteString(",matchIfMissing")
	}
	sb.WriteString(")")
	return sb.String()
}

/********************************* OnBean ************************************/

// onBean represents a condition that checks for the existence of beans
// matching a specific selector in the application context.
type onBean struct {
	beanID gs.BeanID // Bean selector used to find matching beans
}

// OnBean creates a condition that evaluates to true if at least one bean
// matches the specified type and optional name.
func OnBean[T any](name ...string) gs.Condition {
	return &onBean{beanID: gs.BeanIDFor[T](name...)}
}

// OnBeanID creates a condition that evaluates to true if at least one
// bean matches the provided selector.
func OnBeanID(beanID gs.BeanID) gs.Condition {
	return &onBean{beanID: beanID}
}

// Matches checks if there is at least one matching bean in the context.
func (c *onBean) Matches(ctx gs.ConditionContext) (bool, error) {
	beans, err := ctx.Find(c.beanID)
	if err != nil {
		return false, errutil.Explain(err, "condition %s matches error", c)
	}
	return len(beans) > 0, nil
}

func (c *onBean) String() string {
	return fmt.Sprintf("OnBean(selector=%s)", c.beanID)
}

/***************************** OnMissingBean *********************************/

// onMissingBean represents a condition that evaluates to true if no bean
// matches the specified selector in the context.
type onMissingBean struct {
	beanID gs.BeanID // Bean selector for finding beans
}

// OnMissingBean creates a condition that evaluates to true if no bean
// matches the given type and optional name.
func OnMissingBean[T any](name ...string) gs.Condition {
	return &onMissingBean{beanID: gs.BeanIDFor[T](name...)}
}

// OnMissingBeanID creates a condition that evaluates to true if no bean
// matches the provided selector.
func OnMissingBeanID(beanID gs.BeanID) gs.Condition {
	return &onMissingBean{beanID: beanID}
}

// Matches returns true if no beans matching the selector are found.
func (c *onMissingBean) Matches(ctx gs.ConditionContext) (bool, error) {
	beans, err := ctx.Find(c.beanID)
	if err != nil {
		return false, errutil.Explain(err, "condition %s matches error", c)
	}
	return len(beans) == 0, nil
}

func (c *onMissingBean) String() string {
	return fmt.Sprintf("OnMissingBean(selector=%s)", c.beanID)
}

/***************************** OnSingleBean **********************************/

// onSingleBean represents a condition that checks if exactly one bean
// matches the specified selector in the context.
type onSingleBean struct {
	beanID gs.BeanID // Bean selector for finding beans
}

// OnSingleBean creates a condition that evaluates to true if exactly one bean
// matches the given type and optional name.
func OnSingleBean[T any](name ...string) gs.Condition {
	return &onSingleBean{beanID: gs.BeanIDFor[T](name...)}
}

// OnSingleBeanID creates a condition that evaluates to true if exactly
// one bean matches the provided selector.
func OnSingleBeanID(beanID gs.BeanID) gs.Condition {
	return &onSingleBean{beanID: beanID}
}

// Matches returns true if exactly one bean matching the selector is found.
func (c *onSingleBean) Matches(ctx gs.ConditionContext) (bool, error) {
	beans, err := ctx.Find(c.beanID)
	if err != nil {
		return false, errutil.Explain(err, "condition %s matches error", c)
	}
	return len(beans) == 1, nil
}

func (c *onSingleBean) String() string {
	return fmt.Sprintf("OnSingleBean(selector=%s)", c.beanID)
}

/***************************** OnExpression **********************************/

// onExpression represents a condition that evaluates a custom expression
// in the context. The expression should return a boolean value.
type onExpression struct {
	expression string // Expression string to evaluate
}

// OnExpression creates a condition that evaluates a custom boolean expression.
func OnExpression(expression string) gs.Condition {
	return &onExpression{expression: expression}
}

// Matches evaluates the expression (currently unimplemented).
func (c *onExpression) Matches(ctx gs.ConditionContext) (bool, error) {
	err := errutil.ErrUnimplementedMethod
	return false, errutil.Explain(err, "condition %s matches error", c)
}

func (c *onExpression) String() string {
	return fmt.Sprintf("OnExpression(expression=%s)", c.expression)
}

/********************************** Not ***************************************/

// onNot represents a condition that inverts the result of another condition.
type onNot struct {
	c gs.Condition // The condition to negate.
}

// Not creates a condition that returns the negation of another condition.
func Not(c gs.Condition) gs.Condition {
	if c == nil {
		panic("c cannot be nil")
	}
	return &onNot{c: c}
}

// Matches evaluates the wrapped condition and returns its negation.
func (c *onNot) Matches(ctx gs.ConditionContext) (bool, error) {
	ok, err := c.c.Matches(ctx)
	if err != nil {
		return false, errutil.Explain(err, "condition %s matches error", c)
	}
	return !ok, nil
}

func (c *onNot) String() string {
	return fmt.Sprintf("Not(%s)", c.c)
}

/********************************** Or ***************************************/

// onOr represents a condition that combines multiple conditions using
// a logical OR operator. It succeeds if at least one condition is satisfied.
type onOr struct {
	conditions []gs.Condition // List of conditions combined with OR
}

func checkConditions(conditions []gs.Condition) {
	for _, c := range conditions {
		if c == nil {
			panic("conditions cannot contains nil")
		}
	}
}

// Or combines multiple conditions using OR logic.
func Or(conditions ...gs.Condition) gs.Condition {
	checkConditions(conditions)
	return &onOr{conditions: conditions}
}

// Matches evaluates all conditions and returns true if at least one is satisfied.
func (g *onOr) Matches(ctx gs.ConditionContext) (bool, error) {
	for _, c := range g.conditions {
		if ok, err := c.Matches(ctx); err != nil {
			return false, errutil.Explain(err, "condition %s matches error", g)
		} else if ok {
			return true, nil
		}
	}
	return false, nil
}

func (g *onOr) String() string {
	return FormatGroup("Or", g.conditions)
}

/********************************* And ***************************************/

// onAnd represents a condition that combines multiple conditions using
// a logical AND operator. It succeeds only if all conditions are satisfied.
type onAnd struct {
	conditions []gs.Condition // List of conditions combined with AND
}

// And combines multiple conditions using AND logic.
func And(conditions ...gs.Condition) gs.Condition {
	checkConditions(conditions)
	return &onAnd{conditions: conditions}
}

// Matches evaluates all conditions and returns true only if all are satisfied.
func (g *onAnd) Matches(ctx gs.ConditionContext) (bool, error) {
	for _, c := range g.conditions {
		ok, err := c.Matches(ctx)
		if err != nil {
			return false, errutil.Explain(err, "condition %s matches error", g)
		} else if !ok {
			return false, nil
		}
	}
	return true, nil
}

func (g *onAnd) String() string {
	return FormatGroup("And", g.conditions)
}

/********************************** None *************************************/

// onNone represents a condition that succeeds only if none of the
// provided conditions are satisfied.
type onNone struct {
	conditions []gs.Condition // List of conditions combined with NONE
}

// None combines multiple conditions using NONE logic.
// Returns true only if all conditions are false.
func None(conditions ...gs.Condition) gs.Condition {
	checkConditions(conditions)
	return &onNone{conditions: conditions}
}

// Matches evaluates all conditions and returns true only if all are false.
func (g *onNone) Matches(ctx gs.ConditionContext) (bool, error) {
	for _, c := range g.conditions {
		if ok, err := c.Matches(ctx); err != nil {
			return false, errutil.Explain(err, "condition %s matches error", g)
		} else if ok {
			return false, nil
		}
	}
	return true, nil
}

func (g *onNone) String() string {
	return FormatGroup("None", g.conditions)
}

/******************************* utilities ***********************************/

// FormatGroup formats a group of conditions (e.g., AND, OR, NONE) as a string
// for debugging and logging purposes.
func FormatGroup(op string, conditions []gs.Condition) string {
	var sb strings.Builder
	sb.WriteString(op)
	sb.WriteString("(")
	for i, c := range conditions {
		if i > 0 {
			sb.WriteString(",")
		}
		_, _ = fmt.Fprint(&sb, c)
	}
	sb.WriteString(")")
	return sb.String()
}
