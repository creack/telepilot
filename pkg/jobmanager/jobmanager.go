// Package jobmanager provides a simple interface to start/stop/manage arbitrary processes.
package jobmanager

import (
	"errors"
	"io"
	"sync"

	"github.com/google/uuid"
)

// Common errors.
var (
	ErrJobNotFound = errors.New("job not found")
)

// JobManager defines the methods available to interact with the lib.
type JobManager interface {
	StartJob(cmd string, args []string) (uuid.UUID, error)
	StopJob(id uuid.UUID) error
	LookupJob(id uuid.UUID) (*Job, error) // Used to for GetJobStatus and to lookup a job owner.
	StreamLogs(id uuid.UUID) (io.Reader, error)
}

// jobManager is the main controller.
type jobManager struct {
	JobManager

	//nolint:unused // TODO: Implement.
	mu   sync.RWMutex
	jobs map[uuid.UUID]*Job
}

// NewJobManager instantiate the job manager.
//
//nolint:ireturn // Expected interface return.
func NewJobManager() JobManager {
	return &jobManager{jobs: map[uuid.UUID]*Job{}}
}
