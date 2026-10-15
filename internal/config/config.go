// Package config stores the DreamHost API key in ~/.config/dhctl/key (0600).
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	keyEnv       = "DREAMHOST_API_KEY"
	configDirEnv = "DHCTL_CONFIG_DIR"
	keyFileName  = "key"
)

// Dir returns the config directory: $DHCTL_CONFIG_DIR or ~/.config/dhctl.
func Dir() (string, error) {
	if d := strings.TrimSpace(os.Getenv(configDirEnv)); d != "" {
		return filepath.Clean(d), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "dhctl"), nil
}

// KeyPath returns the path of the saved API key.
func KeyPath() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, keyFileName), nil
}

// ErrNoKey means neither $DREAMHOST_API_KEY nor the key file is set.
var ErrNoKey = errors.New("no DreamHost API key: run `dhctl auth` or set " + keyEnv)

// LoadKey returns the API key from $DREAMHOST_API_KEY, else the key file.
func LoadKey() (string, error) {
	if k := strings.TrimSpace(os.Getenv(keyEnv)); k != "" {
		return k, nil
	}
	p, err := KeyPath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNoKey
	}
	if err != nil {
		return "", err
	}
	k := strings.TrimSpace(string(b))
	if k == "" {
		return "", ErrNoKey
	}
	return k, nil
}

// SaveKey writes the API key with 0600 permissions in a 0700 directory.
func SaveKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("API key is empty")
	}
	p, err := KeyPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(p, []byte(key+"\n"), 0o600); err != nil {
		return "", err
	}
	return p, os.Chmod(p, 0o600)
}

// RemoveKey deletes the saved key file, if any.
func RemoveKey() error {
	p, err := KeyPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
