package cgroups

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

func getBlockDevices() ([]string, error) {
	// Open /proc/partitions to get a list of block devices.
	f, err := os.Open("/proc/partitions")
	if err != nil {
		return nil, fmt.Errorf("open partitions file: %w", err)
	}
	defer func() { _ = f.Close() }() // Best effort.

	var devices []string
	for scanner := bufio.NewScanner(f); scanner.Scan(); {
		parts := strings.Fields(scanner.Text())
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
	slog.Debug("Block devices found in /proc/partitions.", "block_devices", devices)
	return devices, nil
}
