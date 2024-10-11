package jobmanager

import (
	"errors"
	"fmt"
	"io"
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

// closeGroup freeze/kills the group then checks if it is still in use.
// Returns true when no more process are attached to the group.
func (j *Job) closeGroup() (bool, error) {
	// Freeze the cgroup for good measure.
	if err := os.WriteFile(filepath.Join(j.cgroupPath, "cgroup.freeze"), []byte("1"), 0); err != nil {
		return false, fmt.Errorf("freeze group: %w", err)
	}
	// Kill all processes in the cgroup.
	if err := os.WriteFile(filepath.Join(j.cgroupPath, "cgroup.kill"), []byte("1"), 0); err != nil {
		return false, fmt.Errorf("kill group: %w", err)
	}

	// Assert that the group is empty.
	buf, err := os.ReadFile(filepath.Join(j.cgroupPath, "cgroup.procs"))
	if err != nil {
		return false, fmt.Errorf("lookup group procs: %w", err)
	}

	return len(buf) == 0, nil
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

	// Wait for ~1 second (arbitrary) for the cgroup to be empty.
	const tickerInterval = 10 * time.Millisecond
	const tickerAttempts = 100
	ticker := time.NewTicker(tickerInterval)
	defer ticker.Stop()
	for range tickerAttempts {
		if empty, err := j.closeGroup(); err != nil {
			// Best effort.
			logger.Error("Error closing group.", "error", err)
			return
		} else if !empty {
			// If the groupp is not empty, wait and try again.
			<-ticker.C
			continue
		}

		if err := os.Remove(j.cgroupPath); err == nil {
			// Success.
			return
		} else if !errors.Is(err, syscall.EBUSY) {
			// If we have an error that is not EBUSY, stop here.
			logger.Error("Error removing cgroup on job close.", "error", err)
			return
		}
		// If cleanup failed, go over again freeze/kill/assert/cleanup.
		// If it happens it likely means something violated the cgroup single writer principle.
		logger.Warn("Cgroup still busy while trying to remove it.")
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
	defer func() {
		// NOTE: This must be kept open until *after* the process started.
		if err := cgroupDir.Close(); err != nil {
			// Best effort. Either the process failed to start or started successfully and we don't need it anymore.
			slog.Warn("Error closing cgroup from parent.", "error", err)
		}
	}()
	// Make use of clone3 cgroup arg.
	j.cmd.SysProcAttr.UseCgroupFD = true
	j.cmd.SysProcAttr.CgroupFD = int(cgroupDir.Fd())
	j.cgroupPath = cgroupDir.Name()

	// Use the broadcaster as output for the process.
	j.cmd.Stdout = j.broadcaster
	j.cmd.Stderr = j.broadcaster // NOTE: Merge out/err for simplicity. Should split them for production.

	// Control pipe.
	r, w, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("os.Pipe: %w", err)
	}
	j.cmd.ExtraFiles = []*os.File{w} // NOTE: We don't support setting extra files. Our pipe will always be '3'.

	if err := j.cmd.Start(); err != nil {
		// NOTE: We don't set a special status for 'failed to start' as this state
		// will be discarded and garbage collected. Never surfaced to the user.
		// When we implement listing, it may be interesting to add.
		j.close()
		_, _ = r.Close(), w.Close() // Best effort.
		return fmt.Errorf("start init process: %w", err)
	}
	_ = w.Close() // Best effort. Needs to be closed before the ReadAll and after Start.
	j.status = pb.JobStatus_JOB_STATUS_RUNNING
	go j.wait()

	startErrBuf, err := io.ReadAll(r)
	_ = r.Close() // Best effort.
	if err != nil {
		return fmt.Errorf("read control pipe: %w", err)
	}
	if len(startErrBuf) != 0 {
		return fmt.Errorf("start job process: %w", errors.New(string(startErrBuf))) //nolint:err113 // Expected.
	}

	return nil
}
