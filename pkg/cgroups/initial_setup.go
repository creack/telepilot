package cgroups

import (
	"fmt"
	"os"
	"path"
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
	// Make sure the base cgroup exists.
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

	// NOTE: If the file doesn't exist, something is wrong, don't attempt to create it.
	if err := os.WriteFile(subtreeControlPath, []byte("+cpu +io +memory"), filePerm); err != nil {
		return fmt.Errorf("enable subtree controls: %w", err)
	}

	return nil
}
