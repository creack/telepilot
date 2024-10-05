package jobmanager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/google/uuid"

	pb "go.creack.net/telepilot/api/v1"
	"go.creack.net/telepilot/pkg/broadcaster"
)

// Job represent an individual job.
type Job struct {
	mu sync.RWMutex

	// Immutable fields. Publicly accessible.
	ID    uuid.UUID
	Owner string

	// Underlying command.
	cmd *exec.Cmd

	// In the context of the assignment, we store all the output in memory
	// and merge stdout/stderr.
	// Not suitable for production as the output can easily cause an OOM crash.
	// Should also split stdout/stderr to allow for more control.
	output *bytes.Buffer

	// Status.
	status pb.JobStatus

	// Log Broadcaster.
	broadcaster *broadcaster.Broadcaster

	// Wait chan, closed when the process ends.
	waitChan chan struct{}
}

func newJob(owner, cmd string, args []string) *Job {
	j := &Job{
		ID:    uuid.New(),
		Owner: owner,
		cmd:   exec.Command(cmd, args...),

		output:      bytes.NewBuffer(nil),
		broadcaster: broadcaster.NewBroadcaster(),

		waitChan: make(chan struct{}),
	}

	return j
}

// historicalSink consumes the given reader (output broadcast)
// and populates the in-memory buffer.
func (j *Job) historicalSink(r io.Reader) {
	buf := make([]byte, 32*1024) //nolint:mnd // Default value from io.Copy, reasonable.
loop:
	n, err := r.Read(buf)
	if err != nil {
		// Nothing to do, broadcast terminated. Ignore error.
		_ = err
		return
	}
	j.mu.Lock()
	_, _ = j.output.Write(buf[:n]) // Can't fail beside OOM.
	j.mu.Unlock()
	goto loop
}

// wait for the underlying process. Broadcast the end via waitChan
// and close the given resources.
func (j *Job) wait(closers ...io.Closer) {
	defer func() {
		j.mu.Lock()
		j.status = pb.JobStatus_JOB_STATUS_EXITED
		j.mu.Unlock()
		close(j.waitChan)
		_ = closeAll(closers...) // Can't fail.
	}()
	if err := j.cmd.Wait(); err != nil {
		// TODO: Consider doing something with the error, store it in the state maybe.
		// Nothing to do with it for now, discarding it.
		_ = err
		return
	}
}

// NOTE: Expected to be called before being shared. Not locked.
func (j *Job) start() error {
	// NOTE: Using background context as we don't want to be bound to the request's one.
	ctx := context.Background()

	// Create a pipe for the outputs.
	r, w := io.Pipe()
	j.cmd.Stdout = w
	j.cmd.Stderr = j.cmd.Stdout // NOTE: Merge out/err for simplicity. Should split them for production.

	// Start the broadcaster.
	go func() { _ = j.broadcaster.Run(ctx, r) }() // Can't fail as we read from a pipe we contorl.

	// Subscribe the in-memory buffer to keep historical logs.
	rc := j.broadcaster.SubscribePipe(ctx)
	go j.historicalSink(rc)

	if err := j.cmd.Start(); err != nil {
		// NOTE: We don't set a special status for 'failed to start' as this state
		// will be discarded and garbage collected. Never surfaced to the user.
		close(j.waitChan)
		_ = closeAll(j.broadcaster, rc, r, w)
		return fmt.Errorf("start process: %w", err)
	}
	j.status = pb.JobStatus_JOB_STATUS_RUNNING
	go j.wait(j.broadcaster, rc, r, w)

	return nil
}

func closeAll(closers ...io.Closer) error {
	if len(closers) == 0 {
		return nil
	}
	errs := make([]error, 0, len(closers))
	for _, c := range closers {
		errs = append(errs, c.Close())
	}
	return errors.Join(errs...)
}
