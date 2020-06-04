package json

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

var (
	// typeRegistry contains globally registered types for JSON encoding/decoding.
	typeRegistry = newTypes()
)

// RegisterType registers a type for Amino-compatible interface encoding in the global type
// registry. These types will be encoded with a type wrapper `{"type":"<type>","value":<value>}`
// regardless of which interface they are wrapped in.
//
// Should only be called in init() functions, as it panics on error.
func RegisterType(name string, _type interface{}) {
	if _type == nil {
		panic("cannot register nil type")
	}
	err := typeRegistry.register(name, reflect.ValueOf(_type).Type())
	if err != nil {
		panic(err)
	}
}

// types is a type registry. It is safe for concurrent use.
type types struct {
	sync.RWMutex
	byType map[reflect.Type]string
	byName map[string]reflect.Type
}

// newTypes creates a new type registry.
func newTypes() types {
	return types{
		byType: map[reflect.Type]string{},
		byName: map[string]reflect.Type{},
	}
}

// registers the given type with the given name. The name and type must not be registered already.
func (t *types) register(name string, rt reflect.Type) error {
	t.Lock()
	defer t.Unlock()
	if name == "" {
		return errors.New("name cannot be empty")
	}
	if _, ok := t.byName[name]; ok {
		return fmt.Errorf("a type with name %q is already registered", name)
	}
	if _, ok := t.byType[rt]; ok {
		return fmt.Errorf("the type %v is already registered", rt)
	}
	t.byName[name] = rt
	t.byType[rt] = name
	return nil
}

// lookup looks up a type from a name, or nil if not registered.
func (t *types) lookup(name string) reflect.Type {
	t.RLock()
	defer t.RUnlock()
	return t.byName[name]
}

// name looks up the name of a type, or empty if not registered. Unwraps pointers as necessary.
func (t *types) name(rt reflect.Type) string {
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	t.RLock()
	defer t.RUnlock()
	return t.byType[rt]
}
