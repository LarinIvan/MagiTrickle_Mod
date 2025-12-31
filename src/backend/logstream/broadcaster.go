package logstream

import (
	"sync"
)

// Broadcaster implements io.Writer and broadcasts messages to subscribers
type Broadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan []byte]struct{}
}

// NewBroadcaster creates a new Broadcaster
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subscribers: make(map[chan []byte]struct{}),
	}
}

// Write implements io.Writer interface
func (b *Broadcaster) Write(p []byte) (n int, err error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.subscribers) == 0 {
		return len(p), nil
	}

	// Make a copy of the data to avoid race conditions if p is reused
	data := make([]byte, len(p))
	copy(data, p)

	for ch := range b.subscribers {
		// Non-blocking send to avoid holding up the logger
		select {
		case ch <- data:
		default:
			// If client is slow, we drop the message for them
		}
	}

	return len(p), nil
}

// Subscribe returns a channel that receives log messages
func (b *Broadcaster) Subscribe() chan []byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan []byte, 100) // Buffer to handle bursts
	b.subscribers[ch] = struct{}{}
	return ch
}

// Unsubscribe removes a subscriber
func (b *Broadcaster) Unsubscribe(ch chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.subscribers[ch]; ok {
		delete(b.subscribers, ch)
		close(ch)
	}
}
