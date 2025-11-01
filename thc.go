package thc

import (
	"strconv"
	"sync"

	"github.com/google/uuid"
	"github.com/kyleraywed/thc/thc_errs"
)

type FuncMap map[string]func()

type container struct {
	identity  string
	removedID string
	data      map[string]any
	mutex     sync.RWMutex

	auditHook FuncMap
}

type Key[T any] struct {
	identity string
	mapKey   string
}

// Number of records in the underlying map
func (c *container) Len() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return len(c.data)
}

// String representation
func (c *container) String() string {
	return "Length: " + strconv.Itoa(c.Len())
}

// Initialize identities, underlying map, and auditHook handler.
func NewTHC(handler FuncMap) *container {
	return &container{
		identity:  uuid.NewString(),
		removedID: uuid.NewString(),
		data:      make(map[string]any),
		auditHook: handler,
	}
}

// Store a value, get a key
func Store[T any](c *container, input T) (Key[T], error) {
	switch any(input).(type) {
	case container:
		if any(input).(container).identity == c.identity {
			var zero Key[T]
			return zero, thc_errs.ErrStoreSelf
		}
	}

	newKey := uuid.NewString()

	c.mutex.Lock()
	c.data[newKey] = input
	c.mutex.Unlock()

	if fn, ok := c.auditHook["Store"]; ok {
		fn()
	}

	return Key[T]{identity: c.identity, mapKey: newKey}, nil
}

// Fetch a value with key, get type-casted value
func Fetch[T any](c *container, key Key[T]) (T, error) {
	var zero T

	if key.identity == c.removedID {
		return zero, thc_errs.ErrDeletedValue
	}
	if c.identity != key.identity {
		return zero, thc_errs.ErrIdentMismatch
	}

	c.mutex.RLock()
	val, ok := c.data[key.mapKey]
	c.mutex.RUnlock()
	if !ok {
		return zero, thc_errs.ErrValNotFound
	}

	casted, ok := val.(T)
	if !ok {
		return zero, thc_errs.ErrTypeCast
	}

	if fn, ok := c.auditHook["Fetch"]; ok {
		fn()
	}

	return casted, nil
}

// Update a value (must be same type)
func Update[T any](c *container, key Key[T], input T) error {
	switch any(input).(type) {
	case container:
		if any(input).(container).identity == c.identity {
			return thc_errs.ErrStoreSelf
		}
	}

	if key.identity == c.removedID {
		return thc_errs.ErrDeletedValue
	}
	if c.identity != key.identity {
		return thc_errs.ErrIdentMismatch
	}

	c.mutex.Lock()
	c.data[key.mapKey] = input
	c.mutex.Unlock()

	if fn, ok := c.auditHook["Update"]; ok {
		fn()
	}

	return nil
}

// Remove a value, invalidate key
func Remove[T any](c *container, key *Key[T]) error {
	if key.identity == c.removedID {
		return thc_errs.ErrDeletedValue
	}
	if c.identity != key.identity {
		return thc_errs.ErrIdentMismatch
	}

	c.mutex.Lock()
	_, ok := c.data[key.mapKey]
	if !ok {
		c.mutex.Unlock()
		return thc_errs.ErrMissingValue
	}
	delete(c.data, key.mapKey)
	c.mutex.Unlock()

	key.identity = c.removedID

	if fn, ok := c.auditHook["Remove"]; ok {
		fn()
	}

	return nil
}
