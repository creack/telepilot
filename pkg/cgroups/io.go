package cgroups

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Common errors.
var ErrNoBlockDeviceFound = errors.New("no block devices found")

func getBlockDevices() ([]string, error) {
	// Open /proc/partitions to get a list of block devices.
	buf, err := os.ReadFile("/proc/partitions")
	if err != nil {
		return nil, fmt.Errorf("read partitions file: %w", err)
	}
	var devices []string //nolint:prealloc // False positive, we don't know the size in advance.
	for _, line := range strings.Split(string(buf), "\n") {
		parts := strings.Fields(line)
		if len(parts) != 4 { //nolint:mnd // We expect 4 fields per line. Skip the rest.
			continue
		}
		major, minor, name := parts[0], parts[1], parts[3]
		if major == "major" { // Skip the header line.
			continue
		}

		// Skip any partitions, only keep devices.
		if lastChar := name[len(name)-1]; lastChar >= '0' && lastChar <= '9' {
			continue
		}
		deviceID := fmt.Sprintf("%s:%s", major, minor)
		devices = append(devices, deviceID)
	}
	if len(devices) == 0 {
		return nil, ErrNoBlockDeviceFound
	}

	slog.With("block_devices", devices).Debug("Block devices found in /proc/partitions.")
	return devices, nil
}
