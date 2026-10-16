package run

import (
	"crypto/ed25519"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/mariyzx/capwatch/internal/hunt"
)

func validOptions() Options {
	return Options{
		OCPUs:        2,
		MemoryGB:     16,
		BootVolumeGB: 50,
		Interval:     time.Minute,
		MaxBackoff:   10 * time.Minute,
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		change  func(*Options)
		wantErr bool
	}{
		{"defaults", func(*Options) {}, false},
		{"always free maximum", func(o *Options) { o.OCPUs, o.MemoryGB, o.BootVolumeGB = 4, 24, 200 }, false},
		{"too many ocpus", func(o *Options) { o.OCPUs = 5 }, true},
		{"no ocpus", func(o *Options) { o.OCPUs = 0 }, true},
		{"too much memory", func(o *Options) { o.MemoryGB = 32 }, true},
		{"boot volume too small", func(o *Options) { o.BootVolumeGB = 20 }, true},
		{"interval too short", func(o *Options) { o.Interval = 5 * time.Second }, true},
		{"backoff shorter than interval", func(o *Options) { o.MaxBackoff = 30 * time.Second }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := validOptions()
			tt.change(&opts)

			if err := opts.validate(); (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func writeKeyFiles(t *testing.T) (publicPath, privatePath string) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	sshPublic, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	privatePEM, err := ssh.MarshalPrivateKey(private, "")
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	publicPath = filepath.Join(dir, "id_ed25519.pub")
	privatePath = filepath.Join(dir, "id_ed25519")
	if err := os.WriteFile(publicPath, ssh.MarshalAuthorizedKey(sshPublic), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(privatePath, pem.EncodeToMemory(privatePEM), 0o600); err != nil {
		t.Fatal(err)
	}
	return publicPath, privatePath
}

func TestReadPublicKey(t *testing.T) {
	publicPath, privatePath := writeKeyFiles(t)

	t.Run("public key", func(t *testing.T) {
		key, err := readPublicKey(publicPath)
		if err != nil {
			t.Fatalf("readPublicKey() error = %v", err)
		}
		if !strings.HasPrefix(key, "ssh-ed25519 ") || strings.HasSuffix(key, "\n") {
			t.Errorf("readPublicKey() = %q", key)
		}
	})

	t.Run("private key", func(t *testing.T) {
		_, err := readPublicKey(privatePath)
		if err == nil {
			t.Fatal("readPublicKey() accepted a private key")
		}
		if strings.Contains(err.Error(), "PRIVATE KEY") {
			t.Errorf("error leaks the key content: %v", err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		if _, err := readPublicKey(filepath.Join(t.TempDir(), "missing.pub")); err == nil {
			t.Fatal("readPublicKey() error = nil")
		}
	})
}

func TestRetryMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"known error", fmt.Errorf("launching instance: %w: %w", hunt.ErrOutOfCapacity, errors.New("Out of host capacity.")), "attempt 3: out of capacity, retrying in 1m2s"},
		{"unknown error", errors.New("boom"), "attempt 3: boom, retrying in 1m2s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := retryMessage(hunt.Attempt{Number: 3, Err: tt.err, Wait: 62400 * time.Millisecond})

			if got != tt.want {
				t.Errorf("retryMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}
