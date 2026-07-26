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

package repository

// Op is a comparison operator used in a [Cond]. It is a small, closed set of
// data-store-agnostic operators — the Go-idiomatic equivalent of a Spring Data
// Specification predicate, deliberately not an expression language. A backend
// translates each Op into its own dialect (a SQL WHERE fragment, a Mongo filter
// document, ...); the set is intentionally small so every backend can support
// all of it without partial coverage.
type Op string

const (
	// Eq matches rows where Field equals Value.
	Eq Op = "eq"
	// Ne matches rows where Field is not equal to Value.
	Ne Op = "ne"
	// Gt matches rows where Field is greater than Value.
	Gt Op = "gt"
	// Ge matches rows where Field is greater than or equal to Value.
	Ge Op = "ge"
	// Lt matches rows where Field is less than Value.
	Lt Op = "lt"
	// Le matches rows where Field is less than or equal to Value.
	Le Op = "le"
	// In matches rows where Field is any of the values in Value, which must be
	// a slice.
	In Op = "in"
	// Like matches rows where Field matches the Value pattern (Value carries the
	// wildcards; the backend does not add them).
	Like Op = "like"
)

// Cond is a single filter predicate: Field <Op> Value. Field is a storage
// column/attribute name supplied by the developer (not end-user input), so it
// is trusted the same way a struct tag is; a backend still validates it as an
// identifier before interpolating it into a query.
type Cond struct {
	Field string
	Op    Op
	Value any
}

// Order is a single sort key. Desc selects descending order; the zero value
// sorts ascending.
type Order struct {
	Field string
	Desc  bool
}

// Pageable is an offset/limit window over a result set. A non-positive Limit
// means "no limit" (return everything after Offset); a non-positive Offset
// starts at the first row.
type Pageable struct {
	Offset int
	Limit  int
}

// Query is the framework-neutral read specification handed to
// [Repository.FindAll] and [Repository.FindPage]. All three parts are optional:
// the zero Query selects every row in insertion/natural order. Build one
// directly or with the fluent helpers ([NewQuery], [Query.Where],
// [Query.OrderBy], [Query.Slice]).
type Query struct {
	// Filters are ANDed together. An empty slice matches everything.
	Filters []Cond
	// Sort lists the sort keys in priority order. An empty slice leaves ordering
	// to the backend.
	Sort []Order
	// Page bounds the returned window. The zero Pageable returns everything.
	Page Pageable
}

// NewQuery returns an empty [Query] ready for the fluent builders. It reads more
// clearly at a call site than a bare Query{} literal when conditions follow.
func NewQuery() Query {
	return Query{}
}

// Where appends an ANDed [Cond] and returns the updated Query, so predicates
// can be chained: NewQuery().Where("age", repository.Ge, 18).Where(...).
func (q Query) Where(field string, op Op, value any) Query {
	q.Filters = append(q.Filters, Cond{Field: field, Op: op, Value: value})
	return q
}

// OrderBy appends an ascending sort key and returns the updated Query.
func (q Query) OrderBy(field string) Query {
	q.Sort = append(q.Sort, Order{Field: field})
	return q
}

// OrderByDesc appends a descending sort key and returns the updated Query.
func (q Query) OrderByDesc(field string) Query {
	q.Sort = append(q.Sort, Order{Field: field, Desc: true})
	return q
}

// Slice sets the offset/limit window and returns the updated Query.
func (q Query) Slice(offset, limit int) Query {
	q.Page = Pageable{Offset: offset, Limit: limit}
	return q
}

// Page is one window of a [Repository.FindPage] result: the items in the window
// plus the Total count of rows the filters match across all windows. It is the
// Go-idiomatic equivalent of Spring Data's Page — enough to render a paginator
// without a second round trip.
type Page[T any] struct {
	// Items are the rows in this window (already limited by Offset/Limit).
	Items []T
	// Total is the count of all rows matching the filters, ignoring the window.
	Total int64
	// Offset and Limit echo the requested window, so callers can compute the
	// current page number without re-deriving it from the original Query.
	Offset int
	Limit  int
}

// HasNext reports whether rows matching the filters remain after this window.
// It is false when the window is unbounded (Limit <= 0).
func (p Page[T]) HasNext() bool {
	if p.Limit <= 0 {
		return false
	}
	return int64(p.Offset+p.Limit) < p.Total
}
