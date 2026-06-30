// Package order provides an in-memory implementation of repository.OrderRepository.
// Replace with a real database / Redis / RPC client as needed.
package order

import (
	"strconv"

	"GS_PROJECT_MODULE/internal-domain/domain/order"

	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	gs.Provide(&Repo{orders: make(map[string]*order.Order)})
}

// Query packages filtering criteria for order queries.
// Zero-value fields mean "no restriction" (all zero values match).
type Query struct {
	UserID *int64
	Status *order.Status
	Limit  int
	Offset int
}

// Match returns true if the order satisfies all specified criteria.
func (q *Query) Match(o *order.Order) bool {
	if q.UserID != nil && o.UserID != *q.UserID {
		return false
	}
	if q.Status != nil && o.Status != *q.Status {
		return false
	}
	return true
}

// Repo is an in-memory implementation of repository.OrderRepository.
type Repo struct {
	orders map[string]*order.Order
	nextID int64
}

// Save stores an order and assigns an auto-increment ID.
func (r *Repo) Save(o *order.Order) error {
	r.nextID++
	o.ID = strconv.FormatInt(r.nextID, 10)
	r.orders[o.ID] = o
	return nil
}

// FindByID retrieves an order by its business ID.
func (r *Repo) FindByID(id string) (*order.Order, error) {
	o, ok := r.orders[id]
	if !ok {
		return nil, errutil.Explain(nil, "order %s not found", id)
	}
	return o, nil
}

// Update persists changes to an existing order.
func (r *Repo) Update(o *order.Order) error {
	if _, ok := r.orders[o.ID]; !ok {
		return errutil.Explain(nil, "order %s not found", o.ID)
	}
	r.orders[o.ID] = o
	return nil
}

// Find returns orders matching the given query, with pagination.
// Zero-value fields in the query are treated as "match all".
func (r *Repo) Find(q Query) ([]*order.Order, error) {
	var result []*order.Order
	for _, o := range r.orders {
		if !q.Match(o) {
			continue
		}
		result = append(result, o)
	}
	// Simple pagination
	offset := q.Offset
	if offset >= len(result) {
		return []*order.Order{}, nil
	}
	end := offset + q.Limit
	if end > len(result) || q.Limit <= 0 {
		end = len(result)
	}
	return result[offset:end], nil
}
