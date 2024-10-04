package jobmanager

import "os/exec"

// Job represent an individual job.
type Job struct {
	Owner string
	Cmd   *exec.Cmd
}
