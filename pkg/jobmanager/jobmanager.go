// Package jobmanager provides a simple interface to start/stop/manage arbitrary processes.
package jobmanager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"syscall"

	"github.com/google/uuid"

	pb "go.creack.net/telepilot/api/v1"
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
	j := newJob(owner, cmd, args)

	if err := j.start(); err != nil {
		return uuid.Nil, fmt.Errorf("job start: %w", err)
	}

	// Job started successfully, store it.
	jm.mu.Lock()
	jm.jobs[j.ID] = j
	jm.mu.Unlock()

	return j.ID, nil
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

func (jm *JobManager) StopJob(id uuid.UUID) error {
	j, err := jm.LookupJob(id)
	if err != nil {
		return err
	}
	if err := func() error {
		j.mu.Lock()
		defer j.mu.Unlock()
		if j.status != pb.JobStatus_JOB_STATUS_RUNNING {
			return nil
		}
		if j.cmd.Process != nil {
			// Send the KILL to the process group, not just the process to avoid orphans.
			// TODO: Change to cmd.Process.Kill using pidfd mode once the cgroup are applied. Done in PR #9.
			if err := syscall.Kill(-j.cmd.Process.Pid, syscall.SIGKILL); err != nil {
				if errors.Is(err, syscall.ESRCH) {
					// If the process died as we were about to stop it, nothing to do.
					// Don't set the status as stopped as it exited on it's own.
					// This is an unavoidable "race" as we don't control the child process,
					// it can die after the lock and before the kill. Nothing to worry about though.
					return nil
				}
				return fmt.Errorf("process kill %d: %w", j.cmd.Process.Pid, err)
			}
		}
		j.status = pb.JobStatus_JOB_STATUS_STOPPED
		return nil
	}(); err != nil {
		return err
	}
	<-j.waitChan
	return nil
}

func (jm *JobManager) StreamLogs(ctx context.Context, id uuid.UUID) (io.Reader, error) {
	j, err := jm.LookupJob(id)
	if err != nil {
		return nil, err
	}

	j.mu.Lock()
	defer j.mu.Unlock()
	if j.status != pb.JobStatus_JOB_STATUS_RUNNING {
		return strings.NewReader(j.output.String()), nil
	}

	// Create a pipe for the caller to consume.
	r, w := io.Pipe()

	j.output.Lock()
	output := j.output.String()
	j.broadcaster.Subscribe(w)
	j.output.Unlock()

	// Cleanup routine. When the process dies or when the context is done,
	// close the pipe and unsubscribe from the broadcaster.
	go func() {
		select {
		case <-ctx.Done():
		case <-j.waitChan:
		}
		_, _ = r.Close(), w.Close() // Can't fail.
		j.broadcaster.Unsubscribe(w)
	}()

	return io.MultiReader(strings.NewReader(output), r), nil
}
