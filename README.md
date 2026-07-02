# govisor

![Status: WIP](https://img.shields.io/badge/status-wip-orange)

`govisor` is a lightweight Linux-first supervisor for host processes. It is built for the case where you want one YAML file to define a small group of long-running programs and you want simple lifecycle control without introducing containers or a heavier orchestration layer.

It already fits well for local development, single-VM apps, personal automation, and internal tools where `apply`, `status`, `logs`, and `stop` are enough operational control.

## Good Fits

- Local dev stacks with 2-6 long-running processes.
  Example: API, worker, and a frontend dev server.
- Small side projects deployed on one VM.
  Example: a Go API plus a queue worker plus a cron-like process.
- Background automation on a personal server.
  Example: poller, sync job, webhook listener, log shipper.
- Internal tools where one YAML file should define what should be running.

## Install

Install by module path:

```sh
go install github.com/deltron-fr/govisor/cmd/govisor@latest
```

This places the `govisor` binary in your `GOBIN` or `$(go env GOPATH)/bin`.

To remove it, delete `govisor` from `GOBIN` or `$(go env GOPATH)/bin`.

## Quick Start

Start the supervisor server:

```sh
govisor start
```

Stop it with `Ctrl-C` or by terminating the `govisor start` process.

In another terminal, create a config file:

```yaml
name: dev-stack
processes:
  - name: api
    command: go
    args: ["run", "./cmd/api"]
    environment:
      APP_ENV: development
      PORT: "8080"
    workdir: ./services/api
    restart: on-failure

  - name: worker
    command: go
    args: ["run", "./cmd/worker"]
    environment:
      QUEUE_NAME: background-jobs
    workdir: ./services/worker
    restart: always

  - name: frontend
    command: npm
    args: ["run", "dev"]
    environment:
      NODE_ENV: development
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

Show recent logs for a process:

```sh
govisor logs api
```

Stop all supervised processes:

```sh
govisor stop
```

## Commands

```text
govisor start
govisor apply -f <config.yaml>
govisor status
govisor logs <process-name>
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
    environment:
      APP_ENV: development
      PORT: "8080"
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
| `processes[].environment` | no | Environment variables added to the inherited environment. Configured values override inherited variables with the same name. |
| `processes[].workdir` | no | Working directory for the process. Relative paths are resolved from the config file directory. |
| `processes[].restart` | no | Restart policy: `always`, `never`, `on-failure`, `unless-stopped`. If omitted, the default is `always`. |
| `processes[].shell` | no | If `true`, runs the command through `sh -c`. |

### Example: Native Host Processes

```yaml
name: side-project
processes:
  - name: app
    command: ./bin/app
    environment:
      APP_ENV: production
      HTTP_PORT: "8080"
    workdir: .
    restart: always

  - name: poller
    command: ./bin/poller
    environment:
      POLL_INTERVAL: 30s
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
    environment:
      NODE_ENV: development
      API_BASE_URL: http://localhost:8080
    workdir: ./frontend
    shell: true
    restart: unless-stopped

  - name: tailwind
    command: npm run watch:css
    workdir: ./frontend
    shell: true
    restart: always

  - name: api
    command: go run ./cmd/api --listen "$LISTEN_ADDRESS"
    environment:
      APP_ENV: development
      LISTEN_ADDRESS: 127.0.0.1:8080
    workdir: ./backend
    shell: true
    restart: on-failure
```

### Example: Checking Environment Variables

Direct commands receive configured variables through their process environment:

```yaml
name: environment-check
processes:
  - name: direct-environment
    command: printenv
    args: ["APP_ENV"]
    environment:
      APP_ENV: production
    restart: never
```

After applying the config, `govisor logs direct-environment` should contain:

```text
production
```

Shell-based commands can also expand configured variables. Variables inherited
by the `govisor` server, such as `HOME`, remain available:

```yaml
name: shell-environment-check
processes:
  - name: shell-environment
    command: printf 'app_env=%s port=%s home=%s\n' "$APP_ENV" "$PORT" "$HOME"
    environment:
      APP_ENV: development
      PORT: "8080"
    shell: true
    restart: never
```

Run `govisor logs shell-environment` to verify the configured values and the
inherited `HOME` value. Environment-variable expansion in `command` is performed
by the shell only when `shell: true`; direct commands should read variables from
their process environment.

## Runtime Paths

`govisor` uses per-user runtime and state directories.

```text
Socket: $XDG_RUNTIME_DIR/govisor/govisor.sock
Logs:   $XDG_STATE_HOME/govisor/logs/
```

When `XDG_RUNTIME_DIR` is unset or empty, the socket falls back to:

```text
~/.local/state/govisor/run/govisor.sock
```

When `XDG_STATE_HOME` is unset or empty, logs fall back to:

```text
~/.local/state/govisor/logs/
```

Each process writes to `<logs-directory>/<process-name>.log`. Govisor creates
the required runtime and log directories automatically. The server and CLI must
resolve the same socket path, so they should run with the same
`XDG_RUNTIME_DIR` value.

## Logging

`govisor` writes stdout and stderr for each process into its own log file.

Retrieve the most recent log output for a supervised process with:

```sh
govisor logs <process-name>
```

The command returns up to the last 4 KiB of the process's current log file. The
supervisor must be running and the named process must have been applied first.

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
- Configured `environment` values are added to the supervisor's inherited environment for both direct and shell-based commands.
- Relative `workdir` paths are resolved from the YAML file location.
- Restart backoff starts at 1 second and grows up to 30 seconds.
- When the server shuts down, it sends `SIGTERM` to supervised processes and attempts a graceful stop.

## Notes

- A configuration name can only be applied once while the server is running.
- Use process names that are unique across all applied configurations.
- This project is best treated as Linux-first today.

## Planned Additions

- configurable log and socket locations
- `depends_on` process dependency graphs
- more complete log rotation and retention
- cgroup-based memory controls on Linux

## Development

For local development, run:

```sh
make help
```

Build the binary:

```sh
make build
```

Run a command:

```sh
make run ARGS='status'
```

Start, inspect, and stop a background server:

```sh
make start
make status
make stop
```

Show recent logs for a supervised process:

```sh
make logs PROCESS='api'
```

Run tests with:

```sh
make test
```

## Contributing

For small changes:

1. Fork the repo and create a branch.
2. Make the change.
3. Run `go test ./...`.
4. Open a pull request with the reasoning behind the change.

For larger changes, opening an issue first is usually the better path so the direction is clear before implementation.
