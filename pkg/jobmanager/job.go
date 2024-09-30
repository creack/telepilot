package jobmanager

import (
	"os/exec"
)

// JobStatus enum type.
type JobStatus string

// JobStatus enum values.
const (
	JobStatusRunning JobStatus = "running"
	JobStatusStopped JobStatus = "stopped"
	JobStatusExited  JobStatus = "exited"
)

// Job represent an individual job.
type Job struct {
	*exec.Cmd
}
