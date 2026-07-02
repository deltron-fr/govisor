BIN_DIR := bin
BIN := $(BIN_DIR)/govisor
SERVER_LOG := $(BIN_DIR)/govisor.out
SERVER_PID := $(BIN_DIR)/govisor.pid

.DEFAULT_GOAL := help

.PHONY: help build run start stop status logs test clean

help:
	@printf '%s\n' \
		'Available targets:' \
		'  make build                         Build the govisor binary' \
		'  make run ARGS='\''status'\''            Build and run govisor with ARGS' \
		'  make start                         Build and start the server in the background' \
		'  make stop                          Stop the background server using the pid file' \
		'  make status                        Show whether the background server pid is running' \
		'  make logs PROCESS='\''api'\''           Show recent logs for a supervised process' \
		'  make test                          Run all tests' \
		'  make clean                         Remove build artifacts'

build:
	@mkdir -p $(BIN_DIR)
	@go build -o $(BIN) ./cmd/govisor

run: build
	@exec ./$(BIN) $(ARGS)

start: build
	@if [ -f $(SERVER_PID) ] && kill -0 "$$(cat $(SERVER_PID))" 2>/dev/null; then \
		echo "server already running with pid $$(cat $(SERVER_PID))"; \
	else \
		nohup ./$(BIN) start >$(SERVER_LOG) 2>&1 & echo $$! > $(SERVER_PID); \
		echo "server started with pid $$(cat $(SERVER_PID))"; \
		echo "log: $(SERVER_LOG)"; \
	fi

stop:
	@if [ -f $(SERVER_PID) ] && kill -0 "$$(cat $(SERVER_PID))" 2>/dev/null; then \
		kill "$$(cat $(SERVER_PID))"; \
		rm -f $(SERVER_PID); \
		echo "server stopped"; \
	else \
		rm -f $(SERVER_PID); \
		echo "server is not running"; \
	fi

status:
	@if [ -f $(SERVER_PID) ] && kill -0 "$$(cat $(SERVER_PID))" 2>/dev/null; then \
		echo "server running with pid $$(cat $(SERVER_PID))"; \
	else \
		echo "server is not running"; \
	fi

logs: build
	@if [ -z "$(PROCESS)" ]; then \
		echo "usage: make logs PROCESS=<process-name>" >&2; \
		exit 2; \
	fi
	@./$(BIN) logs "$(PROCESS)"

test:
	@go test ./...

clean:
	@rm -rf $(BIN_DIR)
