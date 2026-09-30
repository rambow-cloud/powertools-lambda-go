package http

import "sync"

// Store synchronizes its keys. Callers own synchronization of mutable values.
// Each invocation has a fresh Store; Router.Shared survives warm invocations.
type Store struct {
	mu     sync.RWMutex
	values map[string]any
}

func (s *Store) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.values == nil {
		s.values = make(map[string]any)
	}
	s.values[key] = value
}
func (s *Store) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.values[key]
	return value, ok
}
func (s *Store) Has(key string) bool { _, ok := s.Get(key); return ok }
func (s *Store) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.values[key]
	delete(s.values, key)
	return ok
}
func (s *Store) Entries() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]any, len(s.values))
	for key, value := range s.values {
		result[key] = value
	}
	return result
}
