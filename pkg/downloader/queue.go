package downloader

import (
	"sync"
)

// QueueController regulates the number of simultaneously downloading games (Steam-style)
type QueueController struct {
	mu             sync.RWMutex
	maxActiveGames int
}

// NewQueueController creates a queue regulator. Default is 1 active game at a time.
func NewQueueController(maxActive int) *QueueController {
	if maxActive <= 0 {
		maxActive = 1
	}
	return &QueueController{
		maxActiveGames: maxActive,
	}
}

// SetMaxActiveGames updates the concurrency threshold
func (qc *QueueController) SetMaxActiveGames(max int) {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	if max <= 0 {
		max = 1
	}
	qc.maxActiveGames = max
}

// GetMaxActiveGames returns the concurrency threshold
func (qc *QueueController) GetMaxActiveGames() int {
	qc.mu.RLock()
	defer qc.mu.RUnlock()
	return qc.maxActiveGames
}

// CanStartNext checks if another game can begin downloading
func (qc *QueueController) CanStartNext(currentActive int) bool {
	qc.mu.RLock()
	defer qc.mu.RUnlock()
	return currentActive < qc.maxActiveGames
}
