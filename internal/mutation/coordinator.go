package mutation

import "sync"

// Coordinator serializes canonical and durable review transactions while
// allowing consistent reads between mutations.
type Coordinator struct {
	mu sync.RWMutex
}

// NewCoordinator creates an independent mutation coordinator.
func NewCoordinator() *Coordinator {
	return &Coordinator{}
}

// Lock begins an exclusive mutation transaction.
func (c *Coordinator) Lock() {
	c.mu.Lock()
}

// Unlock ends an exclusive mutation transaction.
func (c *Coordinator) Unlock() {
	c.mu.Unlock()
}

// RLock begins a consistent read transaction.
func (c *Coordinator) RLock() {
	c.mu.RLock()
}

// RUnlock ends a consistent read transaction.
func (c *Coordinator) RUnlock() {
	c.mu.RUnlock()
}
