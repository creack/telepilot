package broadcaster_test

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"go.creack.net/telepilot/pkg/broadcaster"
)

type nopCloser struct {
	w io.Writer
}

func (nopCloser) Close() error { return nil }

func (nc *nopCloser) Write(buf []byte) (int, error) { return nc.w.Write(buf) }

func (nc *nopCloser) String() string { return nc.w.(interface{ String() string }).String() }

// type testWriter struct {
// 	w  io.WriteCloser
// 	wg sync.WaitGroup
// }

// func (w *testWriter) Write(buf []byte) (int, error) { defer w.wg.Done(); return w.w.Write(buf) }

// func (w *testWriter) Close() error { return w.w.Close() }

// func (w *testWriter) String() string { return w.w.(interface{ String() string }).String() }

func TestBroadcaster(t *testing.T) {
	t.Parallel()

	b := broadcaster.NewBufferedBroadcaster()

	r, w := io.Pipe()
	// tw := &testWriter{w: w}
	b.Subscribe(w)

	go func() {
		defer b.Unsubscribe(w)
		// tw.wg.Add(1)
		fmt.Fprintf(b, "hello!\n")
		// tw.wg.Wait()
	}()

	buf, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("readall: %s.", err)
	}

	if expect, got := "hello!\n", string(buf); expect != got {
		t.Fatalf("Assert fail.\nExpect:\t%s\nGot:\t%s\n", expect, got)
	}
}

func TestBroadcaster2(t *testing.T) {
	t.Parallel()

	b := broadcaster.NewBufferedBroadcaster()
	r, w := io.Pipe()
	// tw := &testWriter{w: w}
	b.Subscribe(w)
	w2 := &nopCloser{&strings.Builder{}}
	b.Subscribe(w2)
	go func() {
		defer b.Unsubscribe(w)
		defer b.Unsubscribe(w2)
		// tw.wg.Add(1)
		// w2.wg.Add(1)
		fmt.Fprintf(b, "hello!\n")
		// tw.wg.Wait()
		// w2.wg.Wait()
	}()
	buf, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("readall: %s.", err)
	}

	if expect, got := "hello!\n", string(buf); expect != got {
		t.Fatalf("Assert fail1.\nExpect:\t%s\nGot:\t%s\n", expect, got)
	}
	if expect, got := "hello!\n", w2.String(); expect != got {
		t.Fatalf("Assert fail2.\nExpect:\t%s\nGot:\t%s\n", expect, got)
	}
}
