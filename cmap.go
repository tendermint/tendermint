package common

import "sync"

// CMap is a goroutine-safe map
type CMap struct {
	m map[string]interface{}
	l sync.Mutex
}

func NewCMap() *CMap {
	return &CMap{
		m: make(map[string]interface{}, 0),
	}
}

func (cm *CMap) Set(key string, value interface{}) {
	cm.l.Lock()
	defer cm.l.Unlock()
	cm.m[key] = value
}

func (cm *CMap) Get(key string) interface{} {
	cm.l.Lock()
	defer cm.l.Unlock()
	return cm.m[key]
}

func (cm *CMap) Has(key string) bool {
	cm.l.Lock()
	defer cm.l.Unlock()
	_, ok := cm.m[key]
	return ok
}

func (cm *CMap) Delete(key string) {
	cm.l.Lock()
	defer cm.l.Unlock()
	delete(cm.m, key)
}

func (cm *CMap) Size() int {
	cm.l.Lock()
	defer cm.l.Unlock()
	return len(cm.m)
}

func (cm *CMap) Clear() {
	cm.l.Lock()
	defer cm.l.Unlock()
	cm.m = make(map[string]interface{}, 0)
}

func (cm *CMap) Values() []interface{} {
	cm.l.Lock()
	defer cm.l.Unlock()
	items := []interface{}{}
	for _, v := range cm.m {
		items = append(items, v)
	}
	return items
}
