//go:build !windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func (b *Bridge) getPrintersPlatform() ([]string, error) {
	// lpstat -a | cut -d ' ' -f 1
	cmd := exec.Command("sh", "-c", "lpstat -a | cut -d ' ' -f 1")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	var printers []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			printers = append(printers, trimmed)
		}
	}
	return printers, nil
}

func (b *Bridge) printPDFPlatform(printerName, filePath string) error {
	// lp -d printer filename
	cmd := exec.Command("lp", "-d", printerName, filePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("lp error: %v, output: %s", err, string(out))
	}
	return nil
}
