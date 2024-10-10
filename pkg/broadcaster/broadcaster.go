package broadcaster

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

type client struct {
	w    io.Writer
	msgs chan string
	done chan struct{}
}

func (c *client) Close() error {
	// TODO: Consider making the timeout configurable.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond) //nolint:mnd // Arbitrary duration.
	defer cancel()

	close(c.msgs)
	select {
	case <-ctx.Done():
		return ctx.Err() //nolint:wrapcheck // No need to wrap here.
	case <-c.done:
		return nil
	}
}

// BufferedBroadcaster is a simplified Broker allowing clients to subscribe while
// itself behaving as a io.Writer.
// The BufferedBroadcaster is itself a io.Writer, any write to it will be sent to
// all the client that called .SubscribeOutput().
// In addition, it buffers everything written to it. .Buffer() can be used
// to access a copy of it.
// .SubscribeOutput() can be used to subscribe and get a copy at once.
//
// The BufferedBroadcaster must be instantiated using NewBufferedBroadcaster() otherwise
// it would be closed and unusable.
//
// Caveats:
//   - The order of writes is not guaranteed.
//   - Unsubscribe will block until all messages of the client have been written
//     or until a hard-set 500ms timeout is reached.
//   - Close doesn't free the buffer. Relying on the GC for that.
//   - No Cap on the in-memory buffer. Can easily cause OOM.
//   - Slow clients will get evicted if their queue grows too much.
type BufferedBroadcaster struct {
	mu sync.Mutex
	// NOTE: As we use a map, the broadcast order is randomized.
	// Consider using a slice instead to be deterministic.
	// When the broadcaster is closed, is set to nil.
	clients map[io.Writer]*client

	buffer *strings.Builder
}

func NewBufferedBroadcaster() *BufferedBroadcaster {
	return &BufferedBroadcaster{
		clients: map[io.Writer]*client{},
		buffer:  &strings.Builder{},
	}
}

func (b *BufferedBroadcaster) Buffer() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func (b *BufferedBroadcaster) SubscribeOutput(w io.Writer) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribe(w)
	return b.buffer.String()
}

func (b *BufferedBroadcaster) subscribe(w io.Writer) {
	if b.clients == nil { // If closed, do nothing.
		return
	}
	c := &client{
		w:    w,
		msgs: make(chan string, 128), //nolint:mnd // Arbitrary size.
		done: make(chan struct{}),
	}
	b.clients[w] = c
	go func() {
		defer close(c.done)
		for msg := range c.msgs {
			if n, err := w.Write([]byte(msg)); err != nil || n != len(msg) {
				return
			}
		}
	}()
}

func (b *BufferedBroadcaster) Unsubscribe(w io.Writer) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.clients == nil { // If closed, do nothing.
		return
	}
	if c, ok := b.clients[w]; ok {
		// TODO: Consider returning an error to surface the timeout.
		if err := c.Close(); err != nil {
			// Best effort.
			slog.Error("Error closing broadcast client.", "error", err)
		}
		delete(b.clients, w)
	}
}

// NOTE: This approach is not ideal, if any client is slow or blocking,
// it will slow/block everyone, including the command itself.
// In a future version, should consider a more advanced setup where
// each client has it's own goroutine/queue.
func (b *BufferedBroadcaster) Write(p []byte) (int, error) {
	b.mu.Lock()

	// Synch write to the in-memory buffer.
	_, _ = b.buffer.Write(p) // Can't fail beside OOM.
	for w, c := range b.clients {
		select {
		case c.msgs <- string(p):
		default:
			// When the buffer is full, it means the client is either
			// blocking or too slow. Evict it.
			go func() {
				if err := c.Close(); err != nil {
					// Best effort.
					slog.Error("Error closing broadcast client.", "error", err)
				}
			}()
			delete(b.clients, w)
		}
	}
	b.mu.Unlock()
	return len(p), nil
}

func (b *BufferedBroadcaster) Close() error {
	b.mu.Lock()
	// As .Close() blocks, call it for all clients in parallel.
	errCh := make(chan error, len(b.clients))
	for _, c := range b.clients {
		go func() { errCh <- c.Close() }()
	}
	errs := make([]error, 0, len(b.clients))
	for range len(b.clients) {
		// NOTE: There is already a timeout in .Close().
		errs = append(errs, <-errCh)
	}
	close(errCh)
	b.clients = nil
	b.mu.Unlock()
	return errors.Join(errs...)
}
