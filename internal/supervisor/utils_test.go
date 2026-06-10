package supervisor

import "testing"

func TestConfigBaseDir(t *testing.T) {
	s := &Supervisor{configFilePath: "/tmp/govisor/config.yaml"}

	if got := s.configBaseDir(); got != "/tmp/govisor" {
		t.Fatalf("configBaseDir() = %q, want %q", got, "/tmp/govisor")
	}
}

func TestConfigBaseDirEmptyPath(t *testing.T) {
	s := &Supervisor{}

	if got := s.configBaseDir(); got != "" {
		t.Fatalf("configBaseDir() = %q, want empty string", got)
	}
}

func TestHasPathSeparator(t *testing.T) {
	tests := map[string]struct {
		input string
		want  bool
	}{
		"plain command": {input: "sleep", want: false},
		"relative path": {input: "./bin/worker", want: true},
		"nested path":   {input: "bin/worker", want: true},
		"absolute path": {input: "/usr/bin/sleep", want: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := hasPathSeparator(tt.input); got != tt.want {
				t.Fatalf("hasPathSeparator(%q) = %t, want %t", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolvePath(t *testing.T) {
	got := resolvePath("bin/worker", "/tmp/govisor")
	if got != "/tmp/govisor/bin/worker" {
		t.Fatalf("resolvePath() = %q, want %q", got, "/tmp/govisor/bin/worker")
	}
}

func TestResolvePathCleansAbsolutePath(t *testing.T) {
	got := resolvePath("/tmp/govisor/../worker", "/ignored")
	if got != "/tmp/worker" {
		t.Fatalf("resolvePath() = %q, want %q", got, "/tmp/worker")
	}
}

func TestResolvePathWithoutBaseDir(t *testing.T) {
	got := resolvePath("./worker", "")
	if got != "worker" {
		t.Fatalf("resolvePath() = %q, want %q", got, "worker")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "", "worker", "fallback"); got != "worker" {
		t.Fatalf("firstNonEmpty() = %q, want %q", got, "worker")
	}
}

func TestFirstNonEmptyAllEmpty(t *testing.T) {
	if got := firstNonEmpty("", ""); got != "" {
		t.Fatalf("firstNonEmpty() = %q, want empty string", got)
	}
}
