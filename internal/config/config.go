package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

const envPrefix = "CAPWATCH_"

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locating config directory: %w", err)
	}
	return filepath.Join(dir, "capwatch", "config.toml"), nil
}

func Load(path string, flags *pflag.FlagSet, target any) error {
	k := koanf.New(".")

	if err := loadFile(k, path); err != nil {
		return err
	}
	if err := k.Load(env.Provider(".", env.Opt{Prefix: envPrefix, TransformFunc: envKey}), nil); err != nil {
		return fmt.Errorf("reading environment: %w", err)
	}
	if err := checkKnown(k, flags); err != nil {
		return err
	}
	if err := k.Load(posflag.Provider(flags, ".", k), nil); err != nil {
		return fmt.Errorf("reading flags: %w", err)
	}

	return k.Unmarshal("", target)
}

func loadFile(k *koanf.Koanf, path string) error {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("config file %s is writable by other users, run: chmod 600 %s", path, path)
	}
	if err := k.Load(file.Provider(path), toml.Parser()); err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	return nil
}

func envKey(key, value string) (string, any) {
	key = strings.ToLower(strings.TrimPrefix(key, envPrefix))
	return strings.ReplaceAll(key, "_", "-"), value
}

func checkKnown(k *koanf.Koanf, flags *pflag.FlagSet) error {
	for _, key := range k.Keys() {
		if flags.Lookup(key) == nil {
			return fmt.Errorf("unknown setting %q in config file or %s* environment variables", key, envPrefix)
		}
	}
	return nil
}
