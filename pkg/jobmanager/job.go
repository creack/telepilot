package jobmanager

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

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

	// Cgroup path used. Used to cleanup when done.
	cgroupPath string

	// Status.
	status   pb.JobStatus
	exitCode int

	// Log Broadcaster.
	// In the context of the assignment, we store all the output in memory
	// and merge stdout/stderr.
	// Not suitable for production as the output can easily cause an OOM crash.
	// Should also split stdout/stderr to allow for more control.
	broadcaster *broadcaster.BufferedBroadcaster

	// Wait chan, closed when the process ends.
	waitChan chan struct{}
}

func newJob(owner, cmd string, args []string) *Job {
	j := &Job{
		ID:    uuid.New(),
		Owner: owner,
		cmd:   exec.Command(cmd, args...),

		broadcaster: broadcaster.NewBufferedBroadcaster(),

		waitChan: make(chan struct{}),
	}

	j.cmd.SysProcAttr = &syscall.SysProcAttr{
		// Set the process to run in it's own pgid.
		// NOTE: Will probably update this to run in it's own session once we
		// get to the cgroups/namespace implementation.
		Setpgid: true,

		// Create the job in namespaces for isolation.
		Cloneflags: syscall.CLONE_NEWPID | // PID namespace.
			syscall.CLONE_NEWNS | // Mount namespace.
			syscall.CLONE_NEWNET, // Network namespace.
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

func (j *Job) close() {
	j.mu.Lock()
	if j.status != pb.JobStatus_JOB_STATUS_STOPPED {
		j.status = pb.JobStatus_JOB_STATUS_EXITED
	}
	if j.cmd.ProcessState != nil {
		j.exitCode = j.cmd.ProcessState.ExitCode()
	}
	close(j.waitChan)
	if e1 := j.broadcaster.Close(); e1 != nil {
		// Best effort.
		slog.Error("Broadcaster closed with error.", "error", e1)
	}
	j.mu.Unlock()

	// NOTE: cgroupPath is immutable and set at start before being shared, can
	// safely be used without lock.
	logger := slog.With("job_id", j.ID.String(), "cgroup_path", j.cgroupPath)

	//nolint:mnd // Wait for ~1 second (arbitrary) for the cgroup to be empty.
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for range 100 {
		// Freeze the cgroup for good measure.
		if err := os.WriteFile(filepath.Join(j.cgroupPath, "cgroup.kill"), []byte("1"), 0); err != nil {
			// Best effort.
			logger.Error("Error freezing cgroup.", "error", err)
			break
		}
		// Kill all processes in the cgroup.
		if err := os.WriteFile(filepath.Join(j.cgroupPath, "cgroup.kill"), []byte("1"), 0); err != nil {
			// Best effort.
			logger.Error("Error killing cgroup.", "error", err)
			break
		}

		// Assert that the group is empty.
		buf, err := os.ReadFile(filepath.Join(j.cgroupPath, "cgroup.procs"))
		if err != nil {
			logger.Error("Error reading cgroup.procs.", "error", err)
			break
		}
		if len(buf) == 0 {
			// Once empty, attempt to cleanup.
			err := os.Remove(j.cgroupPath)
			if err == nil {
				// Success
				return
			}
			// If cleanup failed, go over again freeze/kill/assert/cleanup.
			// If it happens it likely means something violated the cgroup single writer principle.
			logger.Error("Error removing cgroup on job close.", "error", err)
		}
		<-ticker.C
	}
	logger.Error("Timeout trying to cleanup cgroup.")
}

// wait for the underlying process. Broadcast the end via waitChan
// and close the given resources.
func (j *Job) wait() {
	if err := j.cmd.Wait(); err != nil {
		slog.Debug("Process Wait ended with error", "error", err)
	}
	j.close()
}

// NOTE: Expected to be called before being shared. Not locked.
func (j *Job) start() error {
	// Setup the cgroup limits.
	cgroupDir, err := cgroups.New("job-" + j.ID.String())
	if err != nil {
		return fmt.Errorf("setup cgroups for job: %w", err)
	}
	defer func() { _ = cgroupDir.Close() }() // Best effort.
	// Make use of clone3 cgroup arg.
	j.cmd.SysProcAttr.UseCgroupFD = true
	j.cmd.SysProcAttr.CgroupFD = int(cgroupDir.Fd())
	j.cgroupPath = cgroupDir.Name()

	// Use the broadcaster as output for the process.
	j.cmd.Stdout = j.broadcaster
	j.cmd.Stderr = j.broadcaster // NOTE: Merge out/err for simplicity. Should split them for production.

	if err := j.cmd.Start(); err != nil {
		// NOTE: We don't set a special status for 'failed to start' as this state
		// will be discarded and garbage collected. Never surfaced to the user.
		// When we implement listing, it may be interesting to add.
		j.close()
		return fmt.Errorf("start process: %w", err)
	}
	j.status = pb.JobStatus_JOB_STATUS_RUNNING
	go j.wait()

	return nil
}
