package metadata

import (
	"sync"
)

// call represents an active or completed in-flight request.
type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

// SingleFlightGroup represents a class of work and forms a namespace in
// which units of work can be executed with duplicate suppression.
type SingleFlightGroup struct {
	mu sync.Mutex
	m  map[string]*call
}

// NewSingleFlightGroup creates a new SingleFlightGroup instance.
func NewSingleFlightGroup() *SingleFlightGroup {
	return &SingleFlightGroup{
		m: make(map[string]*call),
	}
}

// Do executes and returns the results of the given function, making
// sure that only one execution is in-flight for a given key at a
// time. If a duplicate comes in, the duplicate caller waits for the
// original to complete and receives the same results.
// The return value shared indicates whether v was given to multiple callers.
func (g *SingleFlightGroup) Do(key string, fn func() (interface{}, error)) (v interface{}, err error, shared bool) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err, true
	}
	c := new(call)
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err, false
}

// Forget tells the singleflight to forget about a key.
func (g *SingleFlightGroup) Forget(key string) {
	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()
}
