package cgroups

import (
	"fmt"
	"os"
	"path"
)

const (
	cpuMax    = "50000 100000"              // 50% (quota per perdiod in usec).
	memoryMax = "52428800"                  // 50 MB (in bytes).
	ioMax     = "rbps=1048576 wbps=1048576" // 1 MB/s (in bytes) read/write.
)

// Create the cgroup (v2) if needed and apply the preset limits.
// Open the cgroup itself and return it, the caller is expected to close.
// Needs to be used with clone3.
//
// NOTE: Naive/basic approach for the sake of the exercise.
// Would want something more flexible for production with maybe one type per cgroup type
// with their own settable limits and serialization logic.
func New(name string) (*os.File, error) {
	cgroupPath := path.Join(cgroupBasePath, name)

	// Lookup devices for the I/O limit.
	devices, err := getBlockDevices()
	if err != nil {
		return nil, fmt.Errorf("get block devices: %w", err)
	}

	// Create cgroup directory.
	if err := os.Mkdir(cgroupPath, dirPerm); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("creating cgroup: %w", err)
	}

	// Set CPU limit.
	if err := os.WriteFile(path.Join(cgroupPath, "cpu.max"), []byte(cpuMax), filePerm); err != nil {
		return nil, fmt.Errorf("set cpu.max toggle: %w", err)
	}

	// Set Memory limit.
	if err := os.WriteFile(path.Join(cgroupPath, "memory.max"), []byte(memoryMax), filePerm); err != nil {
		return nil, fmt.Errorf("set memory.max toggle: %w", err)
	}

	// Set I/O limit.
	f, err := os.OpenFile(path.Join(cgroupPath, "io.max"), os.O_WRONLY, filePerm)
	if err != nil {
		return nil, fmt.Errorf("open io.max toggle: %w", err)
	}
	defer func() { f.Close() }() // Best effort.
	for _, elem := range devices {
		if _, err := fmt.Fprintf(f, "%s %s\n", elem, ioMax); err != nil {
			return nil, fmt.Errorf("set io.max toggle for %q: %w", elem, err)
		}
	}

	// Open cgroup directory for the caller to use.
	cgroupDir, err := os.Open(cgroupPath)
	if err != nil {
		return nil, fmt.Errorf("open cgroup dir: %w", err)
	}
	// NOTE: The caller is expected to close.

	return cgroupDir, nil
}
