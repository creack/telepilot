package cgroups

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
)

const (
	CPUMax    = "50000 100000"              // 50% (quota per perdiod in usec).
	MemoryMax = "52428800"                  // 50 MB (in bytes).
	IOMax     = "rbps=1048576 wbps=1048576" // 1 MB/s (in bytes) read/write.
)

// Create the cgroup (v2) if needed and apply the preset limits.
// Open the cgroup itself and return it, the caller is expected to close.
// Needs to be used with clone3.
//
// NOTE: Naive/basic approach for the sake of the exercise.
// Would want something more flexible for production with maybe one type per cgroup type
// with their own settable limits and serialization logic.
func New(name string) (f *os.File, err error) { //nolint:nonamedreturns // Using named return to cleanup in defer.
	cgroupPath := filepath.Join(CgroupBasePath, name)

	// Create cgroup directory.
	if err := os.Mkdir(cgroupPath, dirPerm); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("creating cgroup: %w", err)
	}
	// In case of error, clean up.
	defer func() {
		if err == nil {
			return
		}
		_ = os.Remove(cgroupPath) // Best effort.
	}()

	if err := setCgroupToggles(cgroupPath); err != nil {
		return nil, fmt.Errorf("setCgroupToggles: %w", err)
	}

	// Open cgroup directory for the caller to use.
	cgroupDir, err := os.OpenFile(cgroupPath, syscall.O_DIRECTORY, 0)
	if err != nil {
		return nil, fmt.Errorf("open cgroup dir: %w", err)
	}
	// NOTE: The caller is expected to close.

	return cgroupDir, nil
}

func setCgroupToggles(cgroupPath string) error {
	// Set CPU limit.
	if err := os.WriteFile(filepath.Join(cgroupPath, "cpu.max"), []byte(CPUMax), filePerm); err != nil {
		return fmt.Errorf("set cpu.max toggle: %w", err)
	}

	// Set Memory limit.
	if err := os.WriteFile(filepath.Join(cgroupPath, "memory.max"), []byte(MemoryMax), filePerm); err != nil {
		return fmt.Errorf("set memory.max toggle: %w", err)
	}

	// Set I/O limit.
	// Lookup devices for the I/O limit.
	devices, err := getBlockDevices()
	if err != nil {
		return fmt.Errorf("get block devices: %w", err)
	}
	ioFile, err := os.OpenFile(filepath.Join(cgroupPath, "io.max"), os.O_WRONLY, filePerm)
	if err != nil {
		return fmt.Errorf("open io.max toggle: %w", err)
	}
	defer func() {
		if err := ioFile.Close(); err != nil {
			// Best effort. Either we already failed to set the toggle or we already finished setting it.
			slog.Warn("Error closing io.max cgroup.", "error", err)
		}
	}()
	for _, elem := range devices {
		if _, err := fmt.Fprintf(ioFile, "%s %s\n", elem, IOMax); err != nil {
			return fmt.Errorf("set io.max toggle for %q: %w", elem, err)
		}
	}

	return nil
}
