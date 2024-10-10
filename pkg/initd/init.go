package initd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Init handles the operations within the namespace for the child process
// before executing the target.
// args is expected to be the target process os.Args.
// args[0] being the command, it will be resolved using the PATH env variable.
func Init(args []string) error {
	if len(args) == 0 {
		return errors.New("missing command") //nolint:err113 // No need for fancy error here.
	}

	// Make the new mount namespace private.The avoids propagation to the host.
	if err := syscall.Mount("none", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("mount root as private: %w", err)
	}

	// NOTE: This is where we would:
	// - mknod the /dev devices like tty/ptmx/random/zero/full
	// - mount-bind the target "image"
	// - mount-bind /sys
	// - pivot root to the image
	// - chdir to / or user defined workdir
	// - unmount / clean old (parent) root.
	// Only dealing with /proc for now. Note that this can easily be 'escaped'
	// and for production, the pivot root wouldn't be optional.

	// Remount /proc to reflect the new PID namespace.
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("remount /proc: %w", err)
	}

	cmd, err := exec.LookPath(args[0])
	if err != nil {
		return fmt.Errorf("lookup path for %q: %w", args[0], err)
	}

	return fmt.Errorf("exec: %w", syscall.Exec(cmd, args, os.Environ()))
}
