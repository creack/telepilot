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

  - **Job creation**: Creating new job with preset resource limits and with their own namespace. Jobs will be automatically started.
  - **Job lifecycle**: Stopping (i.e. sending SIGKILL to the process group), and querying the status of jobs.
    - NOTE: As only one caller can `wait` on a process, we need to properly wrap this to support mutiple clients.
  - **Log streaming**: Providing real-time streaming of job logs (stdout and stderr).
    - NOTE: To support multiple clients, we need to implement a broadcast mechanism.

##### IDs

The Job Manager will use UUIDs as primary key to identify jobs. While we could simplify and use the PID, it would make it more difficult to expand later with a distributed system.

##### Namespaces

To acheive isolation, namespaces will be used. We'll use the following `CloneFlags` as part of `SysProcAttr`:
 - syscall.CLONE_NEWPID: PID namespace which will result in the process having it's own set of pid and run as pid 1.
 - syscall.CLONE_NEWNS: Mount namespace which will result in the process having it's own set of mount points. For simplicity, in the exercise, we'll keep the shared parent mountpoints.
 - syscall.CLONE_NEWNET: Network namespace which will result in the process having it's own set of network interfaces. We'll not implement veth pair / iptables so the process will have no connectivity.

##### Cgroups

We'll use the cgroups v2 api to limit resources. Each job will have it's own group with it's iD, i.e. `/sys/fs/cgroup/telepilot/<job_id>`.
To limit resources we'll use the `cpu.max`, `memory.max` and `io.max` toggles.

To support the sub-cgroups, we'll enable the `+cpu`, `+memory` and `+io` controllers in `/sys/fs/cgroup/telepilot/cgroup.subtree_control`.

To place the process in the cgroup, `/sys/fs/cgroup/telepilot/<job_id>` gets open with `os.Open` and the file description passed to `exec.Cmd` using the `UseCgroupFD` and `CgroupFD` fields from `SysProcAttr`.
This leverages `clone(3)` and places the process in the cgroup upon creation.

We'll use 0.5 CPU, 50MB memory and 1MB/s IO limits as hardcoded presets.

To determine the major/minor for device IO limit we will list `/sys/block` and use the block devices numbers from `/sys/block/<dev name>/dev`.

For production, we may also want to consider to implement more toggles for flexibility.

##### Broadcast

A broadcaster will be implemented, to acheive this, we'll use a simplified Broker pattern, any consumer wanting to see the logs will need to first 'subscribe' at which point it will receive any new data.
If no consumer are subscribed, the data gets discarded, we'll need to make sure to always have a subscriber to be able to retrieve logs from the beginning.
Before starting the process, we'll subscribe to the broadcast via an in-memory buffer. When streaming logs, we'll first send the buffer then the live feed.

#### 2. **gRPC API**

The gRPC API exposes the following methods:

- **StartJob**: Creates and starts a new job with preset resource limits.
- **StopJob**: Stops a running job.
- **GetJobStatus**: Retrieves the status of a job.
- **StreamLogs**: Streams logs for a running job.

Example proto definitions (see [api/api.proto](api/api.proto) for full definition):

```proto
service TelePilotService {
    rpc StartJob (StartJobRequest) returns (StartJobResponse);
    rpc StopJob (StopJobRequest) returns (StopJobResponse);
    rpc GetJobStatus (GetJobStatusRequest) returns (GetJobStatusResponse);
    rpc StreamLogs (StreamLogsRequest) returns (stream StreamLogsResponse);
}
```

##### SSL Configuration

The API server will be serve using TLS 1.3 (latest as of the time of the writing), allowing the recommded curves for key exchange:
- X25519
- P521
- P384
- P256
and the recommended ciphersuites:
- CHACHA20_POLY1305_SHA256
- AES_256_GCM_SHA384
- AES_128_GCM_SHA256

Certificates are using the ECDSA signature algorithm.

See https://www.iana.org/assignments/tls-parameters/tls-parameters.xml for more details.

##### mTLS / User management

The users are identified by validated their certificate and using the presented CN part of the Subject field.

The project will come with 3 preset users, by any new user can be added by using `make certs/client-<name>.pem` will generate and sign the new client files.

##### Tradeoffs / Considerations for production:

- The server will use a self-signed root CA shared between client/server. A proper CA should be used with it's private key well guarded. A different CA should be used for the user management and server verification.
- User management is implemented in the Makefile with a pre-set number of user accounts: `alice`, `bob` and `dave`. A proper user management should be implemented.
  - The authorization scheme is very basic: anyone can start jobs and stop/lookup/stream logs only on their own job. A proper authorization scheme should be implemented.
- The resource limits are preset. It should be settable by the user, ideally in a human readable way.

#### 3. CLI

We'll use the popular [github.com/urfave/cli](github.com/urfave/cli) library because of it's simplicity, flexibility and minimum dependencies.

The CLI will provide commands for:

Server:
  - Start the server

The CLI default certs directory is `./certs` and can be changed with `-certs <certs dir>`. The following files are required:
  - `<certs dir>/ca.pem` CA for clients
  - `<certs dir>/server.pem` server certificate
  - `<certs dir>/server-key.pem` server private key

Client:
  - start: Create and sart a job with pre-defined CPU, memory, and I/O limits.
  - stop: Stop a running job (Send SIGKILL to the underlying process group).
  - status: Get the current status and resource usage of a job.
  - logs: Stream logs for a running job. Gets all logs from the beginning and streams them until the process dies.

The CLI defaults to the user 'alice' and looks for the certs in `./certs`. This can be changed with the `-user <name>` and `-certs <certs dir>` flags. For the sake of the exercise, we won't implement flags for each files and always expect the following:
  - `<certs dir>/ca.pem` server's CA
  - `<certs dir>/client-<name>.pem` client certificate
  - `<certs dir>/client-<name>-key.pem` client private key

As it is an excercise, the address for both the client and server will be hardcoded to `localhost:9090`.

Example usage:

Start the server:

```bash
telepilotd
```

Run a job:

```bash
job_id=$(telepilot start /bin/bash -c "echo Hello")
telepilot status "${job_id}"
telepilot logs "${job_id}"
telepilot -user bob logs "${job_id}" # Expected to fail due to invalid authn.
```

### Tradeoffs / Limitations:

- The full output of jobs will be stored in-memory, which can easily cause the service to crash with OOM.
- PTY support is not implemented. Stdout/Stderr are merged with no time tracking.
- The environment variables cannot be set.
- Input is not implemented, no data can be passed to the jobs beyond the initial commandline arguments.
  - This implies signal forwarding is not implemented as well (terminal resize, custom kill, etc).
- Limited Init process will implemented which means that while in it's own Mount namespace, the process can still see and interract with the host mountpoints at the time the process starts
- No veth pair is setup, while in it's own network namespace, the process has no network capability
- No edit is implemented, any change require creating a new job.
- No delete is implemented, as everything is in-memory, jobs exist as long as the service is running.
- No list operation is implemented.

Considerations for production:

- Should consider using an existing solution like Docker/Podman or even Kubernetes.
- Output should leverage the file system to avoid overflow the memory.
- Should support env variables.
- Input should be implemented
  - Requires a 'reverse broadcast' to allow input from multiple clients for a single process.
  - Should implement signal forwarding (and mapping if a client is implemented for other OS than linux).
- Stdout/Stderr should be split, log entries should include a timestamp.
- PTY should be implemented
  - The excelent (no bias) lib https://github.com/creack/pty can be used.
- A proper init process should be implemented to unshare/unmount the host file-system and setup the veth pair
  - Need to ensure the init process is statically linked to be able to run in a void chroot.
  - This will require a valid chroot, any exported Docker image can work.
  - Once the veth-pair is setup, the host side needs to configure the iptables to route traffic properly.
- Proper CRUD should be implemented for jobs, including listing.
- Jobs should be able to run in their own user namespace

### Future improvments:

The number of possible features is limitless, but here are some that may be interesting to implement in the future:

- Layered filesystem, ability to commit changes
  - Centralized Hub to push/pull filesystems (let's call them 'images')
- Volume management, isolated ox  r mounting from the host's filesystem.
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
- network is not available and only one interface (loopback) is available
- adding a new mountpoint in one job doesn't impact another

### Resource limitations

As cgroup resource monitoring will not be implemented, will only test that the cgroup toggles are properly created with expected values.

## Code Quality

All files of the project are covered by EditorConfig to handle basic file formatting. See [.editorconfig](.editorconfig) for details.

This ensures everyone has the same indentation style/size, end of lines, charset, whitespaces, etc.

The code quality is enforced via GitHub actions and any Pull Requests not complying will be blocked.

### Golang

GolangCi-Lint is used to enforce the Go's coding style. See [.golangci.yml](.golangci.yml) for details.

All linters are enable with a few exceptions that cause too many false positives.

The `depguard` linter in particular is used to enforce the depdencies contraints from the requirements.

### Protobuf

[Buf](https://buf.build/) will be used to enforce Protobuf coding style.

### YAML

`yamllint` is used to enforce YAML style. See [.yamllint](.yamllint) for details. This is used to validate the GitHub actions.

## Project structure

```
├── .dockerignore                # Docker context settings.
├── .editorconfig                # General editor configuration.
├── .gitattributes               # Settings how to display files in diffs.
├── .github                      # GitHub specifc files.
│   └── workflows                # GitHub Actions files.
│       └── lint.yaml            # Linter GitHub Action definitions.
├── .gitignore                   # Git ignored file list.
├── .golangci.yml                # Go linter config.
├── .vscode                      # While I don't use VSCode myself, many do, this provides details about the workspace.
│   └── extensions.json          # Recommended extensions for VSCode.
├── .yamllint                    # yamllint config.
├── DESIGN.md                    # This document.
├── LICENSE                      # License details.
├── Makefile                     # Main Makefile to interact with the project to build, run tests, generate certificates, etc.
├── README.md                    # Main public facing docs. Currently empty but will get populated based on this document.
├── api                          # gRPC API dir.
│   └── v1                       # v1 API.
│       ├── api.pb.go            # Protobuf API definition.
│       ├── api.proto            # <generated> go protobuf file.
│       └── api_grpc.pb.go       # <generated> go grpc protobuf file.
├── cmd                          # Sources for the binaries.
│   ├── telepilot                # Sources for the client binary.
│   │   └── main.go              # Boilerplate client loading ssl certs, connecting to server and calling an empty method.
│   └── telepilotd               # Sources for the server binary.
│       └── main.go              # Boilerplate server loading ssl certs, serving the unimplemented API with middleware to handle authn later.
├── go.mod                       # Go dependencies definition.
├── go.sum                       # Go dependencies lock file.
├── make                         # Makefile helpers dir.
│   ├── Dockerfile.cfssl         # Dockerfile to build the cfssl tool used to generate and manage certs.
│   ├── Dockerfile.protobuf      # Dockerfile to build the protoc compiler with the go grpc plugins.
│   ├── cfssl.json               # Basic cfssl config file.
│   ├── csr.json                 # Basic CSR config file for cfssl.
│   ├── docker.mk                # Docker related targets to build dockerfiles.
│   ├── go-lint.mk               # Golang linter targets.
│   ├── mtls.mk                  # mTLS related targets.
│   ├── protobuf-lint.mk         # Protobuf linter targets.
│   └── protobuf.mk              # Protobuf compiler targets.
└── pkg                          # Packages/libs sources.
    └── jobmanager               # Main package providing the job management logic.
        ├── job.go               # Job definition and logic. Nothing implemented yet.
        └── jobmanager.go        # Job Management logic. Provides the main interface definition, but nothing is implemented yet.
```

## Conclusion

The proposed Job Worker Service will allow users to manage isolated, resource-controlled jobs efficiently using cgroups and Linux namespaces. The design is simple, extensible, and secure, with room for future expansion into distributed systems, advanced job scheduling, job migartion and more.
