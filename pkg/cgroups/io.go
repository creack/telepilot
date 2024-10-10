package cgroups

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func getBlockDevices() ([]string, error) {
	sysDevices, err := os.ReadDir("/sys/block")
	if err != nil {
		return nil, fmt.Errorf("readdir /sys/block: %w", err)
	}

	var devices []string //nolint:prealloc // False positive, we don't know the size in advance.
	for _, device := range sysDevices {
		deviceName := device.Name()
		if strings.HasPrefix(deviceName, "loop") {
			continue
		}
		devPath := filepath.Join("/sys/block", deviceName, "dev")

		// Read the major:minor numbers from the dev file
		devContent, err := os.ReadFile(devPath)
		if err != nil {
			return nil, fmt.Errorf("read device id for %q: %w", deviceName, err)
		}
		devices = append(devices, strings.TrimSpace(string(devContent)))
	}
	slog.Debug("Block devices found in /proc/partitions.", "block_devices", devices)
	return devices, nil
}
