package config

import (
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestConfigFileUnmarshalsProcessEnvironment(t *testing.T) {
	input := []byte(`
name: environment-check
processes:
  - name: api
    command: printenv
    args: ["APP_ENV"]
    environment:
      APP_ENV: production
      PORT: "8080"
    restart: never
`)

	var configFile ConfigFile
	if err := yaml.Unmarshal(input, &configFile); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if len(configFile.Processes) != 1 {
		t.Fatalf("Unmarshal() process count = %d, want 1", len(configFile.Processes))
	}

	got := configFile.Processes[0].Env
	want := map[string]string{
		"APP_ENV": "production",
		"PORT":    "8080",
	}

	if len(got) != len(want) {
		t.Fatalf("Unmarshal() environment = %#v, want %#v", got, want)
	}

	for key, wantValue := range want {
		if gotValue := got[key]; gotValue != wantValue {
			t.Fatalf("Unmarshal() environment[%q] = %q, want %q", key, gotValue, wantValue)
		}
	}
}
