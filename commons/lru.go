package commons

import (
	"container/list"
	"sync"
)

// LRUCache provides capacity eviction without a TTL policy. Values remain caller-owned.
type LRUCache[K comparable, V any] struct {
	mu       sync.Mutex
	capacity int
	order    *list.List
	items    map[K]*list.Element
}
type lruEntry[K comparable, V any] struct {
	key   K
	value V
}

// NewLRUCache defaults to 100 entries. Non-positive capacities retain no entries.
func NewLRUCache[K comparable, V any](capacity ...int) *LRUCache[K, V] {
	limit := 100
	if len(capacity) > 0 {
		limit = capacity[0]
	}
	return &LRUCache[K, V]{capacity: limit, order: list.New(), items: make(map[K]*list.Element)}
}
func (c *LRUCache[K, V]) Add(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.capacity <= 0 {
		return
	}
	if item, ok := c.items[key]; ok {
		item.Value = lruEntry[K, V]{key, value}
		c.order.MoveToBack(item)
		return
	}
	c.items[key] = c.order.PushBack(lruEntry[K, V]{key, value})
	if len(c.items) > c.capacity {
		c.remove(c.order.Front())
	}
}
func (c *LRUCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if item, ok := c.items[key]; ok {
		c.order.MoveToBack(item)
		return item.Value.(lruEntry[K, V]).value, true
	}
	var zero V
	return zero, false
}
func (c *LRUCache[K, V]) Has(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.items[key]
	return ok
}
func (c *LRUCache[K, V]) Remove(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if item, ok := c.items[key]; ok {
		c.remove(item)
	}
}
func (c *LRUCache[K, V]) Size() int { c.mu.Lock(); defer c.mu.Unlock(); return len(c.items) }
func (c *LRUCache[K, V]) Clear()    { c.mu.Lock(); defer c.mu.Unlock(); clear(c.items); c.order.Init() }
func (c *LRUCache[K, V]) remove(item *list.Element) {
	delete(c.items, item.Value.(lruEntry[K, V]).key)
	c.order.Remove(item)
}
