// Package jobmanager provides a simple interface to start/stop/manage arbitrary processes.
package jobmanager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

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

	// Job sarted successfully, store it.
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
	j.mu.Lock()
	if j.status != pb.JobStatus_JOB_STATUS_RUNNING {
		j.mu.Unlock()
		return nil
	}
	if j.cmd.Process != nil {
		if err := j.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("process kill: %w", err)
		}
	}
	j.mu.Unlock()
	<-j.waitChan
	j.mu.Lock()
	j.status = pb.JobStatus_JOB_STATUS_STOPPED
	j.mu.Unlock()
	return nil
}

func (jm *JobManager) StreamLogs(ctx context.Context, id uuid.UUID) (io.Reader, error) {
	j, err := jm.LookupJob(id)
	if err != nil {
		return nil, err
	}

	// Create a pipe for the caller to consume.
	r, w := io.Pipe()

	// NOTE: The RLock efectivelly "pauses" the broadcast to the
	// historical output buffer so we'll always have the full
	// uninterrupted output without duplicates.
	j.mu.RLock()
	output := j.output.String()            // Pull a copy of the historical output.
	rc := j.broadcaster.SubscribePipe(ctx) // Subscribe to the broadcast.
	j.mu.RUnlock()

	// Feed the pipe the with historical output.
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		_, _ = io.Copy(w, strings.NewReader(output)) // Best effort.
	}()

	// Feed the pipe with the live logs from the broadcast.
	go func() {
		// Wait for the historical output to be fully written before starting to feed the live logs.
		select {
		case <-ch:
		case <-ctx.Done():
			return
		}
		_, _ = io.Copy(w, rc) // Best effort.
	}()

	// Cleanup routine.
	go func() {
		defer func() {
			// Close the pipe. Best effort.
			_ = r.Close()
			_ = w.Close()
		}()

		// Wait for the process to end.
		select {
		case <-j.waitChan:
		case <-ctx.Done(): // NOTE: Make sure not to return here and close rc.
		}
		// Once ended, close the broadcast pipe.
		_ = rc.Close() // Best effort.

		// Wait for the historical output to be fed to pipe.
		// Important, especially when the process has already exited.
		select {
		case <-ch:
		case <-ctx.Done():
		}
	}()

	return r, nil
}
