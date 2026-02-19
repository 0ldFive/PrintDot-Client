//go:build linux

package main

import (
	"os"
	"strings"
)

func getDeviceID() (string, error) {
	paths := []string{
		"/etc/machine-id",
		"/var/lib/dbus/machine-id",
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		value := strings.TrimSpace(string(data))
		if value != "" {
			return value, nil
		}
	}

	return "", nil
}
