# TelePilot

TelePilot is a coding assignment for Teleport. It is a remote process manager.

## Build

To build the binary, use `make build`. It will create binaries:
  - the client `./bin/telepilot`
  - the server `./bin/telepilotd`

## Tests

Use `make test` to run the tests. This will generate all the necessary files,
including keys and certs then run the test suite.

## Usage

Before getting started, run `make all` to build binaries, generate a CA, server and client keys/certs.

### Server

In one shell, from the reposiroty root, run the server as root: `sudo ./bin/telepilotd`.

See `./bin/telepilotd -h` for options concerning cert directory.

### Client

In a different shell, from the reposiroty root, you can now use the client:

See `./bin/telepilot -h` for detailed usage / flags.

```bash
job_id=$(./bin/telepilot -user alice start sh -c 'sleep 5; echo hello' | tee /dev/stderr)
./bin/telepilot -user alice status "${job_id}"
./bin/telepilot -user alice logs "${job_id}"

./bin/telepilot -user bob stop "${job_id}" # Expected to fail with Permission Denied.
```

## User Management

Running `make mtls` generates 3 clients: `alice`, `bob` and `dave`.

New clients can be generated on the fly by running `make certs/client-<name>.pem`.
