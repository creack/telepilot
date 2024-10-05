//go:build debug
// +build debug

package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"time"
)

// Only compiled in debug mode. Run GC and prints info
// about the process every few seconds.
func init() { //nolint:gochecknoinits // Expected init for debug.
	//nolint // Debug.
	go func() {
	loop:
		runtime.GC()
		debug.FreeOSMemory()

		fmt.Printf("Number of CPUs: %d\n", runtime.NumCPU())
		fmt.Printf("Number of goroutines: %d\n", runtime.NumGoroutine())
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		fmt.Printf("Alloc = %v MiB\n", m.Alloc/1024/1024)
		fmt.Printf("\tTotalAlloc = %v MiB\n", m.TotalAlloc/1024/1024)
		fmt.Printf("\tSys = %v MiB\n", m.Sys/1024/1024)
		fmt.Printf("\tNumGC = %v\n", m.NumGC)
		fmt.Println()
		time.Sleep(5e9)
		goto loop
	}()
}
