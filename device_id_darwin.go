//go:build darwin

package main

import (
	"bytes"
	"os/exec"
	"strings"
)

func getDeviceID() (string, error) {
	out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		return "", err
	}

	lines := bytes.Split(out, []byte("\n"))
	for _, line := range lines {
		text := strings.TrimSpace(string(line))
		if !strings.Contains(text, "IOPlatformUUID") {
			continue
		}
		if idx := strings.Index(text, "="); idx >= 0 {
			value := strings.TrimSpace(text[idx+1:])
			value = strings.Trim(value, "\"")
			if value != "" {
				return value, nil
			}
		}
	}

	return "", nil
}
