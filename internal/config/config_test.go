package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

type settings struct {
	OCPUs    float32       `koanf:"ocpus"`
	Interval time.Duration `koanf:"interval"`
	Name     string        `koanf:"name"`
}

func newFlags(t *testing.T, args ...string) *pflag.FlagSet {
	t.Helper()
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.Float32("ocpus", 2, "")
	flags.Duration("interval", time.Minute, "")
	flags.String("name", "capwatch", "")
	if err := flags.Parse(args); err != nil {
		t.Fatal(err)
	}
	return flags
}

func writeConfig(t *testing.T, content string, perm os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, perm); err != nil {
		t.Fatal(err)
	}
	return path
}

func missingConfig(t *testing.T) string {
	return filepath.Join(t.TempDir(), "config.toml")
}

func TestLoadPrecedence(t *testing.T) {
	file := writeConfig(t, "ocpus = 3\ninterval = \"2m\"\nname = \"from-file\"\n", 0o600)

	tests := []struct {
		name string
		path string
		env  map[string]string
		args []string
		want settings
	}{
		{"defaults", missingConfig(t), nil, nil, settings{2, time.Minute, "capwatch"}},
		{"file over defaults", file, nil, nil, settings{3, 2 * time.Minute, "from-file"}},
		{"env over file", file, map[string]string{"CAPWATCH_OCPUS": "4", "CAPWATCH_INTERVAL": "90s"}, nil, settings{4, 90 * time.Second, "from-file"}},
		{"flags over env", file, map[string]string{"CAPWATCH_OCPUS": "4"}, []string{"--ocpus=1"}, settings{1, 2 * time.Minute, "from-file"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			var got settings

			if err := Load(tt.path, newFlags(t, tt.args...), &got); err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Load() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestLoadRejects(t *testing.T) {
	tests := []struct {
		name string
		path string
		env  map[string]string
	}{
		{"unknown key in file", writeConfig(t, "key_file = \"~/.oci/key.pem\"\n", 0o600), nil},
		{"unknown environment variable", missingConfig(t), map[string]string{"CAPWATCH_OCPU": "4"}},
		{"file writable by others", writeConfig(t, "ocpus = 3\n", 0o666), nil},
		{"invalid toml", writeConfig(t, "ocpus = \n", 0o600), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			var got settings

			if err := Load(tt.path, newFlags(t), &got); err == nil {
				t.Errorf("Load() error = nil, want an error")
			}
		})
	}
}
