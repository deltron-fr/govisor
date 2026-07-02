package client

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDoCopiesSuccessBody(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("ok\n")),
				}, nil
			}),
		},
	}

	var out bytes.Buffer
	req, err := http.NewRequest(http.MethodGet, baseURL+"/status", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	if err := client.do(req, &out); err != nil {
		t.Fatalf("do() error = %v", err)
	}

	if got := out.String(); got != "ok\n" {
		t.Fatalf("do() wrote %q, want %q", got, "ok\n")
	}
}

func TestDoReturnsTrimmedErrorBody(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader("  bad config  \n")),
				}, nil
			}),
		},
	}

	req, err := http.NewRequest(http.MethodPut, baseURL+"/apply", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	err = client.do(req, io.Discard)
	if err == nil {
		t.Fatal("do() error = nil, want error")
	}

	if got := err.Error(); got != "request failed with status 400: bad config" {
		t.Fatalf("do() error = %q, want %q", got, "request failed with status 400: bad config")
	}
}

func TestLogsHandlerRequestsAndWritesProcessLogs(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Fatalf("LogsHandler() method = %q, want %q", req.Method, http.MethodGet)
				}

				if req.URL.Path != "/logs/api" {
					t.Fatalf("LogsHandler() path = %q, want %q", req.URL.Path, "/logs/api")
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("api started\nrequest handled\n")),
				}, nil
			}),
		},
	}

	var out bytes.Buffer
	if err := client.LogsHandler(&out, "api"); err != nil {
		t.Fatalf("LogsHandler() error = %v", err)
	}

	if got, want := out.String(), "api started\nrequest handled\n"; got != want {
		t.Fatalf("LogsHandler() wrote %q, want %q", got, want)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
