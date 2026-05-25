BIN_DIR := bin
SERVER_BIN := $(BIN_DIR)/govisor-server
CLIENT_BIN := $(BIN_DIR)/govisor
SERVER_LOG := $(BIN_DIR)/govisor-server.out
SERVER_PID := $(BIN_DIR)/govisor-server.pid

.DEFAULT_GOAL := help

.PHONY: help build build-server build-client run-server start-server stop-server server-status run-client clean

help:
	@printf '%s\n' \
		'Available targets:' \
		'  make build                         Build server and client binaries' \
		'  make build-server                  Build the server binary' \
		'  make build-client                  Build the client binary' \
		'  make run-server                    Build and run the server in the foreground' \
		'  make start-server                  Build and start the server in the background' \
		'  make stop-server                   Stop the background server using the pid file' \
		'  make server-status                 Show whether the background server pid is running' \
		'  make run-client ARGS='\''status'\''' \
		'                                     Build and run the client with ARGS' \
		'  make clean                         Remove build artifacts'

build: build-server build-client

build-server:
	@mkdir -p $(BIN_DIR)
	@go build -o $(SERVER_BIN) ./cmd/server

build-client:
	@mkdir -p $(BIN_DIR)
	@go build -o $(CLIENT_BIN) ./cmd/client

run-server: build-server
	@exec ./$(SERVER_BIN)

start-server: build-server
	@if [ -f $(SERVER_PID) ] && kill -0 "$$(cat $(SERVER_PID))" 2>/dev/null; then \
		echo "server already running with pid $$(cat $(SERVER_PID))"; \
	else \
		nohup ./$(SERVER_BIN) >$(SERVER_LOG) 2>&1 & echo $$! > $(SERVER_PID); \
		echo "server started with pid $$(cat $(SERVER_PID))"; \
		echo "log: $(SERVER_LOG)"; \
	fi

stop-server:
	@if [ -f $(SERVER_PID) ] && kill -0 "$$(cat $(SERVER_PID))" 2>/dev/null; then \
		kill "$$(cat $(SERVER_PID))"; \
		rm -f $(SERVER_PID); \
		echo "server stopped"; \
	else \
		rm -f $(SERVER_PID); \
		echo "server is not running"; \
	fi

server-status:
	@if [ -f $(SERVER_PID) ] && kill -0 "$$(cat $(SERVER_PID))" 2>/dev/null; then \
		echo "server running with pid $$(cat $(SERVER_PID))"; \
	else \
		echo "server is not running"; \
	fi

run-client: build-client
	@./$(CLIENT_BIN) $(ARGS)

clean:
	@rm -rf $(BIN_DIR)
