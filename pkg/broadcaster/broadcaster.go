package broadcaster

import (
	"errors"
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
//   - Each write sequencially calls .Write() on the clients, if any client blocks
//     or slows down, it impacts every client as well as the initial caller using Write()
//     on the broadcaster.
type Broadcaster struct {
	mu sync.Mutex
	// NOTE: As we use a map, the broadcast order is randomized.
	// Consider using a slice instead to be deterministic.
	clients map[io.WriteCloser]struct{}
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		clients: map[io.WriteCloser]struct{}{},
	}
}

// Surface the lock as Pause/Unpause to freeze the broadcast.
// This is useful if one of the broadcaster client requires to be read
// from a different goroutine. Typically, when one of the client is an
// in-memory buffer than needs to be accessed on demand.
// Calling Pause() pauses all the clients as well as the use of the broadcast
// calling Write().
// Calling Resume() resumes execution. Calling Pause() without Resume() can lead
// to goroutine leak and/or deadlocks.
func (b *Broadcaster) Pause()  { b.mu.Lock() }
func (b *Broadcaster) Resume() { b.mu.Unlock() }

// If broadcaster is closed, do nothing.
func (b *Broadcaster) Subscribe(w io.WriteCloser) {
	b.mu.Lock()
	b.UnsafeSubscribe(w)
	b.mu.Unlock()
}

// Needs to be gated by Pause/Unpause.
func (b *Broadcaster) UnsafeSubscribe(w io.WriteCloser) {
	if b.clients == nil {
		return
	}
	b.clients[w] = struct{}{}
}

func (b *Broadcaster) Unsubscribe(w io.WriteCloser) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.clients == nil {
		return
	}
	delete(b.clients, w)
}

// NOTE: This approach is not ideal, if any client is slow or blocking,
// it will slow/block everyone, including the command itself.
// In a future version, should consider a more advanced setup where
// each client has it's own goroutine/queue.
func (b *Broadcaster) Write(p []byte) (int, error) {
	b.mu.Lock()
	for c := range b.clients {
		if n, err := c.Write(p); err != nil || n != len(p) {
			// Any errors kicks the client out.
			delete(b.clients, c)
		}
	}
	b.mu.Unlock()
	return len(p), nil
}

func (b *Broadcaster) Close() error {
	b.mu.Lock()
	errs := make([]error, 0, len(b.clients))
	for c := range b.clients {
		errs = append(errs, c.Close())
	}
	b.clients = nil
	b.mu.Unlock()
	return errors.Join(errs...)
}
