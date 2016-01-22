package common

import "sync"

// CMapInt is a goroutine-safe map
type CMapInt struct {
	m map[int]interface{}
	l sync.Mutex
}

func NewCMapInt() *CMapInt {
	return &CMapInt{
		m: make(map[int]interface{}, 0),
	}
}

func (cm *CMapInt) Set(key int, value interface{}) {
	cm.l.Lock()
	defer cm.l.Unlock()
	cm.m[key] = value
}

func (cm *CMapInt) Get(key int) interface{} {
	cm.l.Lock()
	defer cm.l.Unlock()
	return cm.m[key]
}

func (cm *CMapInt) Has(key int) bool {
	cm.l.Lock()
	defer cm.l.Unlock()
	_, ok := cm.m[key]
	return ok
}

func (cm *CMapInt) Delete(key int) {
	cm.l.Lock()
	defer cm.l.Unlock()
	delete(cm.m, key)
}

func (cm *CMapInt) Size() int {
	cm.l.Lock()
	defer cm.l.Unlock()
	return len(cm.m)
}

func (cm *CMapInt) Clear() {
	cm.l.Lock()
	defer cm.l.Unlock()
	cm.m = make(map[int]interface{}, 0)
}

func (cm *CMapInt) Values() []interface{} {
	cm.l.Lock()
	defer cm.l.Unlock()
	items := []interface{}{}
	for _, v := range cm.m {
		items = append(items, v)
	}
	return items
}
