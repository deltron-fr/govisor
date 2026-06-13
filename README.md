# govisor

![Status: WIP](https://img.shields.io/badge/status-wip-orange)

`govisor` is a lightweight Linux-first supervisor for host processes. It is built for the case where you want one YAML file to define a small group of long-running programs and you want simple lifecycle control without introducing containers or a heavier orchestration layer.

It already fits well for local development, single-VM apps, personal automation, and internal tools where `apply`, `status`, and `stop` are enough operational control.

## Good Fits

- Local dev stacks with 2-6 long-running processes.
  Example: API, worker, and a frontend dev server.
- Small side projects deployed on one VM.
  Example: a Go API plus a queue worker plus a cron-like process.
- Background automation on a personal server.
  Example: poller, sync job, webhook listener, log shipper.
- Internal tools where one YAML file should define what should be running.

## Install

The current install flow uses two binaries:

- `govisor` for the CLI
- `govisor-server` for the supervisor server

I plan to simplify this in a future change so installation only requires a single binary.

Install by module path:

```sh
go install github.com/deltron-fr/govisor/cmd/govisor@latest
go install github.com/deltron-fr/govisor/cmd/govisor-server@latest
```

This places the binaries in your `GOBIN` or `$(go env GOPATH)/bin`.

To remove them, delete `govisor` and `govisor-server` from `GOBIN` or `$(go env GOPATH)/bin`.

## Quick Start

Start the supervisor server:

```sh
govisor-server
```

Stop it with `Ctrl-C` or by terminating the `govisor-server` process.

In another terminal, create a config file:

```yaml
name: dev-stack
processes:
  - name: api
    command: go
    args: ["run", "./cmd/api"]
    workdir: ./services/api
    restart: on-failure

  - name: worker
    command: go
    args: ["run", "./cmd/worker"]
    workdir: ./services/worker
    restart: always

  - name: frontend
    command: npm
    args: ["run", "dev"]
    workdir: ./frontend
    restart: unless-stopped
```

Apply it:

```sh
govisor apply -f ./govisor.yaml
```

Check status:

```sh
govisor status
```

Stop all supervised processes:

```sh
govisor stop
```

## Commands

```text
govisor apply -f <config.yaml>
govisor status
govisor stop
```

### Example Status Output

```text
$ govisor status
NAME       STATUS     COMMAND   CREATED    UPDATED
api        RUNNING    go        14:03:11   14:03:12
worker     RUNNING    go        14:03:11   14:03:13
frontend   RUNNING    npm       14:03:11   14:03:11
```

## Configuration

Config files are YAML with this top-level shape:

```yaml
name: my-stack
processes:
  - name: api
    description: optional description
    command: go
    args: ["run", "./cmd/api"]
    workdir: ./services/api
    restart: on-failure
    shell: false
```

### Fields

| Field | Required | Description |
| --- | --- | --- |
| `name` | yes | Name of the config group. |
| `processes` | yes | List of processes to supervise. |
| `processes[].name` | yes | Unique process name. Use unique names in a config. |
| `processes[].description` | no | Human-readable description. |
| `processes[].command` | yes | Executable or command string to run. |
| `processes[].args` | no | Arguments passed to the command when `shell` is `false`. |
| `processes[].workdir` | no | Working directory for the process. Relative paths are resolved from the config file directory. |
| `processes[].restart` | no | Restart policy: `always`, `never`, `on-failure`, `unless-stopped`. If omitted, the default is `always`. |
| `processes[].shell` | no | If `true`, runs the command through `sh -c`. |

### Example: Native Host Processes

```yaml
name: side-project
processes:
  - name: app
    command: ./bin/app
    workdir: .
    restart: always

  - name: poller
    command: ./bin/poller
    workdir: .
    restart: on-failure

  - name: webhook-listener
    command: ./bin/webhook-listener
    workdir: .
    restart: unless-stopped
```

### Example: Shell-Based Commands

```yaml
name: local-dev
processes:
  - name: frontend
    command: npm run dev
    workdir: ./frontend
    shell: true
    restart: unless-stopped

  - name: tailwind
    command: npm run watch:css
    workdir: ./frontend
    shell: true
    restart: always

  - name: api
    command: go run ./cmd/api
    workdir: ./backend
    shell: true
    restart: on-failure
```

## Runtime Paths

Current defaults are:

```text
Socket: /tmp/govisor/govisor.sock
Logs:   /tmp/govisor/log/
```

Each process writes to:

```text
/tmp/govisor/log/<process-name>.log
```

The server socket and logs are currently under `/tmp` due to permission constraints in the current implementation. That location is expected to change.

## Logging

`govisor` writes stdout and stderr for each process into its own log file.

Current rotation behavior is intentionally simple:

- rotation is size-based
- the size threshold is 10 MiB per log file
- the rotation check runs every 45 seconds
- the active file becomes `<name>.1.log`
- a fresh `<name>.log` file is opened

More robust rotation is planned later, including archiving older logs, deleting old logs by age, and supporting safer restart behavior around rotation.

## Process Model

- `govisor` supervises host processes, not containers.
- Commands can be executed directly or through `sh -c` with `shell: true`.
- Relative `workdir` paths are resolved from the YAML file location.
- Restart backoff starts at 1 second and grows up to 30 seconds.
- When the server shuts down, it sends `SIGTERM` to supervised processes and attempts a graceful stop.

## Notes

- Use unique process names in a config. Duplicate names are not rejected yet.
- This project is best treated as Linux-first today.

## Planned Additions

- configurable server startup from the CLI
- configurable log and socket locations
- stronger duplicate-name validation
- more complete log rotation and retention
- cgroup-based memory controls on Linux

## Development

For local development, run:

```sh
make help
```

Run tests with:

```sh
go test ./...
```

## Contributing

For small changes:

1. Fork the repo and create a branch.
2. Make the change.
3. Run `go test ./...`.
4. Open a pull request with the reasoning behind the change.

For larger changes, opening an issue first is usually the better path so the direction is clear before implementation.
