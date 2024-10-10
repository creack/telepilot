package cgroups

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strings"
)

const (
	dirPerm  = 0o755
	filePerm = 0o644

	CgroupBasePath = "/sys/fs/cgroup/telepilot"
)

// InitialSetup creates the base cgroup if needed and enables the subtree controls.
//
// TODO: Consider cleaning up when the server dies.
func InitialSetup() error {
	// Make sur the base cgroup exists.
	if _, err := os.Stat(CgroupBasePath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("stat base cgroup: %w", err)
		}
		if err := os.MkdirAll(CgroupBasePath, dirPerm); err != nil {
			return fmt.Errorf("create base cgroup: %w", err)
		}
	}

	// Make sure we have the required controls for the sub cgroups.
	subtreeControlPath := path.Join(CgroupBasePath, "cgroup.subtree_control")

	// Define the controls we need.
	neededControls := map[string]struct{}{
		"cpu":    {},
		"memory": {},
		"io":     {},
	}

	// NOTE: If the file doesn't exist, something is wrong, don't attempt to create it.
	f, err := os.OpenFile(subtreeControlPath, os.O_APPEND|os.O_RDWR, filePerm)
	if err != nil {
		return fmt.Errorf("open subtree control: %w", err)
	}
	defer func() { _ = f.Close() }() // Best effort.

	// Get the existing controls.
	buf, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("read subtree control: %w", err)
	}
	// Remove each existing ones from the needed ones.
	for _, ctrl := range strings.Fields(string(buf)) {
		delete(neededControls, ctrl)
	}

	slog.
		With("current_subtree_controls", strings.TrimSpace(string(buf))).
		With("needed_controls", neededControls).
		Debug("Initial subtree controls.")

	// Enable the ones we still need.
	for ctrl := range neededControls {
		if _, err := fmt.Fprint(f, "+"+ctrl); err != nil {
			return fmt.Errorf("enable %q control: %w", ctrl, err)
		}
	}
	return nil
}
