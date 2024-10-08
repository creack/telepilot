package broadcaster

import (
	"io"
	"sync"
)

// Broadcaster is a simplified Broker allowing clients to subscribe while
// itself behaving as a io.Writer.
// The Broadcaster is itself a io.Writer, any write to it will be sent to
// all the client that called .Subscribe() (or .UnsafeSubscribe()).
//
// Caveats:
//   - The order of writes is not guaranteed.
type Broadcaster struct {
	mu sync.Mutex
	// NOTE: As we use a map, the broadcast order is randomized.
	// Consider using a slice instead to be deterministic.
	clients map[io.WriteCloser]chan string
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		clients: map[io.WriteCloser]chan string{},
	}
}

// If broadcaster is closed, do nothing.
func (b *Broadcaster) Subscribe(w io.WriteCloser) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.clients == nil { // If closed, do nothing.
		return
	}
	ch := make(chan string, 128) //nolint:mnd // Arbitrary size.
	b.clients[w] = ch
	go func() {
		for msg := range ch {
			if n, err := w.Write([]byte(msg)); err != nil || n != len(msg) {
				return
			}
		}
	}()
}

func (b *Broadcaster) Unsubscribe(w io.WriteCloser) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.clients == nil { // If closed, do nothing.
		return
	}
	if ch, ok := b.clients[w]; ok {
		close(ch)
		delete(b.clients, w)
	}
	_ = w.Close()
}

// NOTE: This approach is not ideal, if any client is slow or blocking,
// it will slow/block everyone, including the command itself.
// In a future version, should consider a more advanced setup where
// each client has it's own goroutine/queue.
func (b *Broadcaster) Write(p []byte) (int, error) {
	b.mu.Lock()
	for w, ch := range b.clients {
		select {
		case ch <- string(p):
		default:
			close(ch)
			_ = w.Close() // Best effort.
			delete(b.clients, w)
		}
	}
	b.mu.Unlock()
	return len(p), nil
}

func (b *Broadcaster) Close() error {
	b.mu.Lock()
	for w, ch := range b.clients {
		close(ch)
		_ = w.Close() // Best effort.
	}
	b.clients = nil
	b.mu.Unlock()
	return nil
}
