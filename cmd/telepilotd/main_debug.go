//go:build debug
// +build debug

package main

import (
	"log/slog"
	"os"
	"runtime"
	"runtime/debug"
	"time"
)

// Only compiled in debug mode. Run GC and prints info
// about the process every few seconds.
func init() { //nolint:gochecknoinits // Expected init for debug.
	logger := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(logger))

	//nolint // Debug.
	go func() {
		for {
			runtime.GC()
			debug.FreeOSMemory()

			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			slog.
				With("num_goroutine", runtime.NumGoroutine()).
				// NOTE: Could cast to float64 to get more details, but not needed.
				With("mem_alloc_mib", m.Alloc/1024/1024).
				With("mem_sys_mib", m.Sys/1024/1024).
				With("mem_total_alloc_mib", m.TotalAlloc/1024/1024).
				Debug("Stats.")
			time.Sleep(5 * time.Second)
		}
	}()
}
