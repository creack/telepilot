// Package jobmanager provides a simple interface to start/stop/manage arbitrary processes.
package jobmanager

import (
	"errors"
	"io"
	"os/exec"
	"sync"

	"github.com/google/uuid"
)

// Common errors.
var (
	ErrJobNotFound = errors.New("job not found")
)

// JobManager defines the methods available to interact with the lib.
//
// TODO: Make this a struct instead. Currently just a blank mock to highlight the upcoming methods.
type JobManager interface {
	StartJob(owner, cmd string, args []string) (uuid.UUID, error)
	StopJob(id uuid.UUID) error
	LookupJob(id uuid.UUID) (*Job, error) // Used to for GetJobStatus and to lookup a job owner.
	StreamLogs(id uuid.UUID) (io.Reader, error)
}

// jobManager is the main controller.
type jobManager struct {
	JobManager

	mu   sync.RWMutex
	jobs map[uuid.UUID]*Job
}

// NewJobManager instantiate the job manager.
//
//nolint:ireturn // Expected interface return.
func NewJobManager() JobManager {
	return &jobManager{jobs: map[uuid.UUID]*Job{}}
}

func (jm *jobManager) StartJob(owner, cmd string, args []string) (uuid.UUID, error) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	jobID := uuid.New()
	// NOTE: Focusing on Auth at the moment, the jobs are just placeholders, not actually started.
	jm.jobs[jobID] = &Job{Owner: owner, Cmd: exec.Command(cmd, args...)}

	return jobID, nil
}

func (jm *jobManager) LookupJob(id uuid.UUID) (*Job, error) {
	jm.mu.RLock()
	j := jm.jobs[id]
	jm.mu.RUnlock()
	if j == nil {
		return nil, ErrJobNotFound
	}
	return j, nil
}
