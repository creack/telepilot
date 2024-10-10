package telepilot_test

import (
	"os"
	"path"
	"strconv"
	"strings"
	"testing"

	"go.creack.net/telepilot/pkg/cgroups"
)

// As described in the design doc, simple set of test
// to ensure the cgroups are created with the proper value.
func TestCgroups(t *testing.T) {
	t.Parallel()

	ts, ctx := newTestServer(t)

	// Create a Job.
	jobID, err := ts.alice.StartJob(ctx, "sleep", []string{"5"})
	noError(t, err, "Start job.")
	t.Cleanup(func() { noError(t, ts.alice.StopJob(ctx, jobID), "Cleanup stop job.") })

	cgroupJobPath := path.Join(cgroups.CgroupBasePath, "job-"+jobID)

	// Make sure the cgroup is in use. Note that because the process
	// runs in it's own pid namespace, we can't dump echo $$ to get it's pid.
	// When we implement a more detailed job status, we could include the real pid
	// and assert it.
	t.Run("pid", func(t *testing.T) {
		t.Parallel()
		buf, err := os.ReadFile(path.Join(cgroupJobPath, "cgroup.procs"))
		noError(t, err, "read job cpu.procs")
		pid := strings.TrimSpace(string(buf))
		if _, err := strconv.Atoi(pid); err != nil || pid == "" {
			t.Fatalf("Unexpected job output: %q not a pid (%v).", pid, err)
		}
	})

	// Assert the cgroup values.
	t.Run("cpu", func(t *testing.T) {
		t.Parallel()
		buf, err := os.ReadFile(path.Join(cgroupJobPath, "cpu.max"))
		noError(t, err, "read job cpu.max")
		assert(t, cgroups.CPUMax, strings.TrimSpace(string(buf)), "invalid cpu.max value")
	})
	t.Run("memory", func(t *testing.T) {
		t.Parallel()
		buf, err := os.ReadFile(path.Join(cgroupJobPath, "memory.max"))
		noError(t, err, "read job memory.max")
		assert(t, cgroups.MemoryMax, strings.TrimSpace(string(buf)), "invalid memory.max value")
	})
	t.Run("io", func(t *testing.T) {
		t.Parallel()
		// NOTE: This test will be effective only if there is a block device on the machine.
		//        GitHub actions shared workers have 2.
		buf, err := os.ReadFile(path.Join(cgroupJobPath, "io.max"))
		noError(t, err, "read job io.max")
		for _, line := range strings.Split(strings.TrimSpace(string(buf)), "\n") {
			if !strings.Contains(line, cgroups.IOMax) {
				t.Fatalf("Invalid io.max entry: %q.", line)
			}
		}
	})
}
