package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds this device's own identity and settings.
type Config struct {
	DeviceID string `json:"device_id"`
	Name     string `json:"name"`
	Port     int    `json:"port"`
}

func configPath() (string, error) {
	dir, err := hmpDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// LoadConfig reads this device's config.json. Returns an error if it
// doesn't exist yet — a device must be set up (name chosen, ID generated)
// before it can run, which is the installer's job.
func LoadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("hmp: not set up yet — run 'hmp -setup' first")
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveConfig writes this device's config.json, overwriting any existing one.
func SaveConfig(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// GenerateDeviceID creates a random UUID-v4-style identifier for this device.
// Generated once at setup time and never changes afterward.
func GenerateDeviceID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("hmp: failed to generate device id: %w", err)
	}

	// Set UUID version (4) and variant bits per RFC 4122.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// SetupDevice generates a fresh identity for this device and saves it.
// Called by the installer (or `hmp -setup`) exactly once.
func SetupDevice(name string, port int) (*Config, error) {
	id, err := GenerateDeviceID()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		DeviceID: id,
		Name:     name,
		Port:     port,
	}

	if err := SaveConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
