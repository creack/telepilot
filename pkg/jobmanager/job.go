package jobmanager

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"

	"github.com/google/uuid"

	pb "go.creack.net/telepilot/api/v1"
	"go.creack.net/telepilot/pkg/broadcaster"
	"go.creack.net/telepilot/pkg/cgroups"
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

		output:      bytes.NewBuffer(nil),
		broadcaster: broadcaster.NewBroadcaster(),

		waitChan: make(chan struct{}),
	}

	// Set the process to run in it's own pgid.
	// NOTE: Will probably update this to run in it's own session once we
	// get to the cgroups/namespace implementation.
	j.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:     true,
		UseCgroupFD: true,
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
	defer func() {
		j.mu.Lock()
		if j.status != pb.JobStatus_JOB_STATUS_STOPPED {
			j.status = pb.JobStatus_JOB_STATUS_EXITED
		}
		j.exitCode = j.cmd.ProcessState.ExitCode()
		close(j.waitChan)
		_ = j.broadcaster.Close() // Best effort.
		j.mu.Unlock()
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
	// Setup the cgroup limits.
	cgroupDir, err := cgroups.New("job-" + j.ID.String())
	if err != nil {
		return fmt.Errorf("setup cgroups for job: %w", err)
	}
	defer func() { _ = cgroupDir.Close() }() // Best effort.
	j.cmd.SysProcAttr.CgroupFD = int(cgroupDir.Fd())

	// Use the broadcaster as output for the process.
	j.cmd.Stdout = j.broadcaster
	j.cmd.Stderr = j.cmd.Stdout // NOTE: Merge out/err for simplicity. Should split them for production.

	// Subscribe the in-memory buffer to keep historical logs.
	j.broadcaster.Subscribe(&nopCloser{j.output})

	if err := j.cmd.Start(); err != nil {
		// NOTE: We don't set a special status for 'failed to start' as this state
		// will be discarded and garbage collected. Never surfaced to the user.
		// When we implement listing, it may be interesting to add.
		close(j.waitChan)
		_ = j.broadcaster.Close() // Best effort.
		return fmt.Errorf("start process: %w", err)
	}
	j.status = pb.JobStatus_JOB_STATUS_RUNNING
	go j.wait()

	return nil
}

// nopCloser wraps io.Writer and adds a no-op Closer method.
type nopCloser struct{ io.Writer }

func (n *nopCloser) Close() error { return nil }
