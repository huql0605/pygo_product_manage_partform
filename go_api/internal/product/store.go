package product

import (
	"sync"
)

type Store struct {
	mu     sync.RWMutex
	items  map[int]Product
	nextId int
}

func NewStore() *Store {
	return &Store{items: make(map[int]Product), nextId: 1}
}

func (s *Store) List() []Product {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Product, 0, len(s.items))
	for _, p := range s.items {
		out = append(out, p)
	}
	return out
}

func (s *Store) Create(p Product) Product {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.ID = uint(s.nextId)
	s.nextId++
	s.items[int(p.ID)] = p
	return p
}
