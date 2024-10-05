// Package broadcaster provides a simple way to broadcast a reader to many writers.
//
// TODO: Consider splitting in 2 parts: generic channel-only broadcaster and
// []byte broadcaster with support for io.Reader consumer/pipes.
package broadcaster

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
)

// Client wraps the client's channel.
// TODO: Consider using `chan []byte` instead, but then need to
// make sure we send a copy of the slice to the clients.
type Client struct {
	C  <-chan string // Public facing for the caller to receive messages.
	ch chan string   // Internal reference to send/close.
}

// clientPipe wraps the client as a io.ReadCloser.
type clientPipe struct {
	io.ReadCloser
	client      *Client      // Reference to the client.
	broadcaster *Broadcaster // Reference to the broadcaster for unsub on Close.
}

// Close unsubscribes the client from the broadcaster.
func (c *clientPipe) Close() error {
	c.broadcaster.Unsubscribe(c.client)
	return c.ReadCloser.Close() //nolint:wrapcheck // Only error path.
}

// Broadcaster handles the state.
type Broadcaster struct {
	// List of clients.
	mu      sync.RWMutex
	clients map[*Client]struct{}

	// Input chan. Anything coming in from that channel gets broadcasted to all clients.
	broadcast chan string

	// Contol chans.
	register   chan *Client
	unregister chan *Client

	// Close control.
	closed atomic.Uint32
}

// NewBroadcaster initializes the resources for the broadcaster.
func NewBroadcaster() *Broadcaster {
	// TODO: Instrument/Monitor channel buffer sizes, should never more than 1, maybe 2 on high load.
	// Not stricly necessary to use buffered chans but it ensures we never block.
	const bufferSize = 128
	return &Broadcaster{
		clients:    map[*Client]struct{}{},
		broadcast:  make(chan string, bufferSize),
		register:   make(chan *Client, bufferSize),
		unregister: make(chan *Client, bufferSize),
	}
}

// controllLoop handles register/unregister operations.
func (b *Broadcaster) controllLoop(ctx context.Context) {
loop:
	select {
	case <-ctx.Done():
		return
	case client, ok := <-b.register:
		if !ok { // Closed broadcaster.
			return
		}
		b.mu.Lock() // Protect write access to clients map.
		b.clients[client] = struct{}{}
		b.mu.Unlock()
	case client := <-b.unregister:
		b.mu.Lock() // Protect write access to clients map.
		if _, ok := b.clients[client]; ok {
			delete(b.clients, client)
			close(client.ch)
		}
		b.mu.Unlock()
	}
	goto loop
}

// broadcastLoop handles the actual message broadcast.
func (b *Broadcaster) broadcastLoop(ctx context.Context) {
loop:
	select {
	case <-ctx.Done():
		return
	case message, ok := <-b.broadcast:
		if !ok { // Closed broadcaster.
			return
		}
		b.mu.RLock()
		for client := range b.clients {
			// Try to the send message to the client,
			// if the client's send buffer is full, unregister it.
			select {
			case client.ch <- message:
			default:
				// NOTE: Unlikely, but potentially could block
				// until the controlLoop goroutine clears the queue.
				b.unregister <- client
			}
		}
		b.mu.RUnlock()
	}
	goto loop
}

// Run starts the broadcast, consuming the given reader.
// Returns the r.Read() errror.
// If the broadcaster is already closed, doesn't do anything.
func (b *Broadcaster) Run(ctx context.Context, r io.Reader) error {
	if b.closed.Load() == 1 {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go b.controllLoop(ctx)
	go b.broadcastLoop(ctx)

	buf := make([]byte, 32*1024) //nolint:mnd // Default value from io.Copy, reasonable.
loop:
	n, err := r.Read(buf)
	if err != nil {
		return err //nolint:wrapcheck // Only error path, no need to wrap.
	}
	if b.closed.Load() == 1 {
		return nil
	}
	select {
	case <-ctx.Done():
	case b.broadcast <- string(buf[:n]):
	}
	goto loop
}

// SubscribePipe subscribes to the broadcast, returning the read side of a pipe.
//
// If the broadcaster is already closed, doesn't do anything and returns nil.
//
// NOTE: The caller is expected to close the client when done.
func (b *Broadcaster) SubscribePipe(ctx context.Context) io.ReadCloser {
	client := b.Subscribe()
	if client == nil {
		return nil
	}

	r, w := io.Pipe()
	go func() {
	loop:
		select {
		case <-ctx.Done():
			return
		case msg := <-client.C:
			if _, err := w.Write([]byte(msg)); err != nil {
				// Best effort. Any error terminates the loop.
				return
			}
		}
		goto loop
	}()

	return &clientPipe{
		ReadCloser:  r,
		client:      client,
		broadcaster: b,
	}
}

// Subscribe to the broadcast.
//
// If the broadcaster is already closed, doesn't do anything and returns nil.
func (b *Broadcaster) Subscribe() *Client {
	if b.closed.Load() == 1 {
		return nil
	}

	ch := make(chan string, 128) //nolint:mnd // Arbitrary size. Buffered to avoid blocking.
	c := &Client{
		C:  ch,
		ch: ch,
	}
	b.register <- c

	return c
}

// Unsubscribe from the broadcast.
// If the broadcaster is already closed, doesn't do anything.
func (b *Broadcaster) Unsubscribe(c *Client) {
	if b.closed.Load() == 1 {
		return
	}

	b.unregister <- c
}

// Close the broadcaster.
// Can't fail. Has an error return to comply with io.Closer.
func (b *Broadcaster) Close() error {
	if !b.closed.CompareAndSwap(0, 1) {
		return nil
	}

	b.mu.RLock()
	for c := range b.clients {
		b.unregister <- c
	}
	b.mu.RUnlock()

	close(b.register)
	close(b.unregister)
	close(b.broadcast)

	return nil
}
