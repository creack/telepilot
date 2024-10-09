package jobmanager

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"syscall"

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
	output *lockWriteCloser

	// Status.
	status   pb.JobStatus
	exitCode int

	// Log Broadcaster.
	// broadcaster *broadcaster.Broadcaster
	broadcaster *broadcaster.Broadcaster

	// Wait chan, closed when the process ends.
	waitChan chan struct{}
}

func newJob(owner, cmd string, args []string) *Job {
	j := &Job{
		ID:    uuid.New(),
		Owner: owner,
		cmd:   exec.Command(cmd, args...),

		output:      &lockWriteCloser{Builder: &strings.Builder{}},
		broadcaster: broadcaster.NewBroadcaster(),

		waitChan: make(chan struct{}),
	}

	// Set the process to run in it's own pgid.
	// NOTE: Will probably update this to run in it's own session once we
	// get to the cgroups/namespace implementation.
	j.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	return j
}

func (j *Job) Status() pb.JobStatus {
	j.mu.RLock()
	s := j.status
	j.mu.RUnlock()
	return s
}

func (j *Job) ExitCode() int {
	j.mu.RLock()
	c := j.exitCode
	j.mu.RUnlock()
	return c
}

// wait for the underlying process. Broadcast the end via waitChan
// and close the given resources.
func (j *Job) wait() {
	if err := j.cmd.Wait(); err != nil {
		slog.Debug("Process Wait ended with error", "error", err)
	}

	j.mu.Lock()
	defer j.mu.Unlock()
	if j.status != pb.JobStatus_JOB_STATUS_STOPPED {
		j.status = pb.JobStatus_JOB_STATUS_EXITED
	}
	j.exitCode = j.cmd.ProcessState.ExitCode()
	close(j.waitChan)
	if err := j.broadcaster.Close(); err != nil {
		// Best effort.
		slog.Error("Broadcaster closed with error.", "error", err)
	}
}

// NOTE: Expected to be called before being shared. Not locked.
func (j *Job) start() error {
	// Use the broadcaster as output for the process.
	j.cmd.Stdout = j.broadcaster
	j.cmd.Stderr = j.broadcaster // NOTE: Merge out/err for simplicity. Should split them for production.

	// Subscribe the in-memory buffer to keep historical logs.
	j.broadcaster.Subscribe(j.output)

	if err := j.cmd.Start(); err != nil {
		// NOTE: We don't set a special status for 'failed to start' as this state
		// will be discarded and garbage collected. Never surfaced to the user.
		// When we implement listing, it may be interesting to add.
		close(j.waitChan)
		if e1 := j.broadcaster.Close(); e1 != nil {
			// Best effort.
			slog.Error("Broadcaster closed with error.", "error", e1)
		}
		return fmt.Errorf("start process: %w", err)
	}
	j.status = pb.JobStatus_JOB_STATUS_RUNNING
	go j.wait()

	return nil
}

// lockWriteCloser wraps io.Writer and adds a lock and a no-op Closer method.
type lockWriteCloser struct {
	sync.Mutex
	*strings.Builder
}

func (n *lockWriteCloser) Write(buf []byte) (int, error) {
	n.Lock()
	defer n.Unlock()
	return n.Builder.Write(buf) //nolint:wrapcheck // No need for wrap here.
}

func (n *lockWriteCloser) Close() error { return nil }
