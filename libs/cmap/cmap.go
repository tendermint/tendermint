package cmap

import (
	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
)

// CMap is a goroutine-safe map
type CMap struct {
	m map[string]interface{}
	l tmsync.RWMutex
}

func NewCMap() *CMap {
	return &CMap{
		m: make(map[string]interface{}),
	}
}

func (cm *CMap) Set(key string, value interface{}) {
	cm.l.Lock()
	cm.m[key] = value
	cm.l.Unlock()
}

// GetOrSet returns the existing value if present. Otherwise, it stores `newValue` and returns it.
// The loaded result is true if the value was loaded, false if stored.
func (cm *CMap) GetOrSet(key string, newValue interface{}) (value interface{}, loaded bool) {
	cm.l.Lock()
	defer cm.l.Unlock()
	if v, ok := cm.m[key]; ok {
		return v, true
	}
	cm.m[key] = newValue
	return newValue, false
}

func (cm *CMap) Get(key string) interface{} {
	cm.l.RLock()
	val := cm.m[key]
	cm.l.RUnlock()
	return val
}

func (cm *CMap) Has(key string) bool {
	cm.l.RLock()
	_, ok := cm.m[key]
	cm.l.RUnlock()
	return ok
}

func (cm *CMap) Delete(key string) {
	cm.l.Lock()
	delete(cm.m, key)
	cm.l.Unlock()
}

func (cm *CMap) Size() int {
	cm.l.RLock()
	size := len(cm.m)
	cm.l.RUnlock()
	return size
}

func (cm *CMap) Clear() {
	cm.l.Lock()
	cm.m = make(map[string]interface{})
	cm.l.Unlock()
}

func (cm *CMap) Keys() []string {
	cm.l.RLock()
	keys := make([]string, 0, len(cm.m))
	for k := range cm.m {
		keys = append(keys, k)
	}
	cm.l.RUnlock()
	return keys
}

func (cm *CMap) Values() []interface{} {
	cm.l.RLock()
	items := make([]interface{}, 0, len(cm.m))
	for _, v := range cm.m {
		items = append(items, v)
	}
	cm.l.RUnlock()
	return items
}
