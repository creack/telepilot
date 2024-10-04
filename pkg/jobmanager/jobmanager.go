// Package jobmanager provides a simple interface to start/stop/manage arbitrary processes.
package jobmanager

import (
	"errors"
	"os/exec"
	"sync"

	"github.com/google/uuid"
)

// Common errors.
var (
	ErrJobNotFound = errors.New("job not found")
)

// JobManager is the main controller.
type JobManager struct {
	mu   sync.RWMutex
	jobs map[uuid.UUID]*Job
}

// NewJobManager instantiate the job manager.
func NewJobManager() *JobManager {
	return &JobManager{jobs: map[uuid.UUID]*Job{}}
}

func (jm *JobManager) StartJob(owner, cmd string, args []string) (uuid.UUID, error) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	jobID := uuid.New()
	// NOTE: Focusing on Auth at the moment, the jobs are just placeholders, not actually started.
	jm.jobs[jobID] = &Job{Owner: owner, Cmd: exec.Command(cmd, args...)}

	return jobID, nil
}

func (jm *JobManager) LookupJob(id uuid.UUID) (*Job, error) {
	jm.mu.RLock()
	j := jm.jobs[id]
	jm.mu.RUnlock()
	if j == nil {
		return nil, ErrJobNotFound
	}
	return j, nil
}
