---
authors: Guillaume J. Charmes (git+guillaume@charmes.net)
---

# TelePilot, the Job Worker Service.

## What

Coding assignment for Level 5 Systems Engineer.

This document describes the plan to implement a **Job Worker Service** that allows users to create, start, stop, query, and stream logs for jobs running on Linux.

Each job runs within their own namespace and with resource limits for **CPU**, **memory**, and **disk I/O** enforced using **cgroups**.

The service will contain the following components:
- The Job Manager library acting as supervisor to own/control the jobs
- The server, using the library and exposing the interface via an API
- The client to communicate with the server that lets users execute and manage jobs

The server and client will communicate using a **gRPC** API and will use **mTLS** for authentication.

## Design Goals

1. **Simplicity**: The system should be easy to use and maintain, with a gRPC API and simple CLI commands.
2. **Isolation**: Use Linux namespaces to isolate processes.
3. **Resource control**: Leverage **cgroups v2** to control CPU, memory, and I/O usage.
4. **Extensibility**: Keep the system modular and open for future enhancements, such as job scheduling and clustering.

## Non-Goals

1. **Job Scheduling**: The current version will not include advanced scheduling algorithms but will support simple create/start/stop semantics.
2. **High Availability**: The service will not be highly available in its initial version but will support future expansion into a distributed architecture.
3. **Deployment**: The project will not be automatically deployed via CI/CD.

## Design

### Components

#### 1. **Worker Library**

A reusable Go library that manages:

  - **Job creation**: Creating new job with specific resource limits and with their own namespace.
  - **Job lifecycle**: Starting, stopping, and querying the status of jobs.
    - NOTE: As only one caller can `wait` on a process, we need to properly wrap this to support mutiple clients.
  - **Log streaming**: Providing real-time streaming of job logs (stdout and stderr).
    - NOTE: To support multiple clients, we need to implement a broadcast mechanism.

The Job Manager will use UUIDs as primary key to identify jobs. While we could simplify and use the PID, it would make it more difficult to expand later with a distributed system.

#### 2. **gRPC API**

The gRPC API exposes the following methods:

- **CreateJob**: Creates a new job with specified resource limits but does not start it.
- **StartJob**: Starts a previously created job.
- **StopJob**: Stops a running job.
- **GetJobStatus**: Retrieves the status of a job.
- **StreamLogs**: Streams logs for a running job.

Example proto definitions (see [api/api.proto](api/api.proto) for full definition):

```proto
service TelePilot {
    rpc CreateJob (CreateJobRequest) returns (CreateJobResponse);
    rpc StartJob (StartJobRequest) returns (StartJobResponse);
    rpc StopJob (StopJobRequest) returns (StopJobResponse);
    rpc GetJobStatus (JobStatusRequest) returns (JobStatusResponse);
    rpc StreamLogs (LogRequest) returns (stream LogEntry);
}
```

The API server will be serve using TLS 1.3 (latest as of the time of the writing), allowing the recommended ciphers

Tradeoffs / Considerations for production:

- The server will use a self-signed root CA. A proper one should be used with they private key well guarded.
- User management is implemented in the Makefile with a pre-set number of user accounts: `alice`, `bob` and `dave`. A proper user management should be implemented.
  - The authorization scheme is very basic: anyone can create jobs and read only their own job data. A proper authorization scheme should be implemented.

#### 3. CLI

The CLI will provide commands for:

Server:
  - Start the server

Cient:
  - create: Create a job with specified CPU, memory, and I/O limits.
  - start: Start a created job.
  - stop: Stop a running job.
  - wait: Wait for a job to end.
  - status: Get the current status and resource usage of a job.
  - logs: Stream logs for a running job. Gets all logs from the beginning and streams them until the process dies.
  - run: Wraps create/start/logs.

Example usage:

Start the server:

```bash
telepilotd
```

Run a job:

```bash
job_id=$(telepilot create --cpu 0.5 --memory 512MB --io 10MB/s -- /bin/bash -c "echo Hello")
telepilot start "${job_id}"
telepilot status "${job_id}"
telepilot logs "${job_id}"
```

or simply:

```bash
telepilot run bash -c "echo Hello"
```

### Tradeoffs / Limitations:

- The full output of jobs will be stored in-memory, which can easily cause the service to crash with OOM.
- PTY support is not implemented.
- The environment variables cannot be set.
- Input is not implemented, no data can be passed to the jobs beyond the initial commandline arguments.
  - This implies signal forwarding is not implemented as well (terminal resize, custom kill, etc).
- Init process is not implemented which means
  - While in it's own PID namespace, the process can still see, but not interract with the host processes
  - While in it's own Mount namespace, the process can still see and interract with the host mountpoints at the time the process starts
- User namespace is basic and only maps the host user (the one running the server) to root
- No veth pair is setup, while in it's own network namespace, the process has no network capability
- No edit is implemented, any change require creating a new job.
- No delete is implemented, as everything is in-memory, jobs exist as long as the service is running.

Considerations for production:

- Should consider using an existing solution like Docker/Podman or even Kubernetes.
- Output should leverage the file system to avoid overflow the memory.
- Should support env variables.
- Input should be implemented
  - Requires a 'reverse broadcast' to allow input from multiple clients for a single process.
  - Should implement signal forwarding (and mapping if a client is implemented for other OS than linux).
- PTY should be implemented
  - The excelent (no bias) lib https://github.com/creack/pty can be used.
- An init process should be implemented to unshare/unmount the host file-system and setup the veth pair
  - Need to ensure the init process is statically linked to be able to run in a void chroot.
  - This will 'hide' the host process from within the PID namespace as the process won't have access to the host's /proc.
  - This will require a valid chroot, any exported Docker image can work.
  - Once the veth-pair is setup, the host side needs to configure the iptable to route traffic properly.
- Proper CRUD should be implemented for jobs.

NOTE: Based on time, or requirements clarification, the init process may be implemented for pid/mount isolation and maybe to enable networking.

### Future improvments:

The number of possible features is limitless, but here are some that may be interesting to implement in the future:

- Layered filesystem, ability to commit changes
  - Centralized Hub to push/pull filesystems (let's call them 'images')
- Volume management, isolated or mounting from the host's filesystem.
- Isolated networking / job network assignment
- Distributed networking
- Secrets management
- Job scaling
- Job dependencies
- Freeze/Unfreeze jobs
- Job migration cross-hosts
- Job resources monitoring
- Job-in-Job support

## Testing

### Authn/Authz

Both the happy and error paths will be tested to ensure that:
- everything works as expected well all is well
- no unathorized access can be performed
- no forbidden operation can be executed by an authorized user

### Namespace isolation

To ensure the isoaltion is working as expected some tests will be implemented to ensure that:
- the job is running with pid 1
- a file created as root within a job end up as the external user on disc
- network is not available and only one interface (loopback) is available
- adding a new mountpoint in one job doesn't impact another

### Resource limitations

As cgroup resource monitoring will not be implemented, will provide some test scripts to run manually with a 3rd party monitoring tool like `htop`.

## Code Quality

All files of the project are covered by EditorConfig to handle basic file formatting. See [.editorconfig](.editorconfig) for details.

This ensures everyone has the same indentation style/size, end of lines, charset, whitespaces, etc.

The code quality is enforced via GitHub actions and any Pull Requests not complying will be blocked.

### Golang

GolangCi-Lint is used to enforce the Go's coding style. See [.golangci.yml](.golangci.yml) for details.

All linters are enable with a few exceptions that cause too many false positives.

The `depguard` linter in particular is used to enforce the depdencies contraints from the requirements.

### Protobuf

Clang-format is used to enforce the Protobuf coding style. See [.clang-format](.clang-format) for details.

### YAML

`yamllint` is used to enforce YAML style. See [.yamllint](.yamllint) for details.

## Conclusion

The proposed Job Worker Service will allow users to manage isolated, resource-controlled jobs efficiently using cgroups and Linux namespaces. The design is simple, extensible, and secure, with room for future expansion into distributed systems, advanced job scheduling, job migartion and more.
