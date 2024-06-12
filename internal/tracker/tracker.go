package tracker

import "sync"

type RequestCounter struct {
	mu       *sync.Mutex
	requests int
}

func (rc *RequestCounter) IncrementRequests() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.requests++
}

func (rc *RequestCounter) GetRequestCount() int {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.requests
}

func NewCounter() *RequestCounter {
	return &RequestCounter{
		mu: &sync.Mutex{},
	}
}
