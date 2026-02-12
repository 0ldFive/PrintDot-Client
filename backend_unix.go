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

func (b *Bridge) printPDFPlatform(printerName, filePath string, options PrintOptions) error {
	args := []string{"-d", printerName}

	if options.Copies > 1 {
		args = append(args, "-n", fmt.Sprintf("%d", options.Copies))
	}

	if options.PageRange != "" {
		args = append(args, "-P", options.PageRange)
	}

	switch strings.ToLower(strings.TrimSpace(options.PageSet)) {
	case "even":
		args = append(args, "-o", "page-set=even")
	case "odd":
		args = append(args, "-o", "page-set=odd")
	}

	switch strings.ToLower(strings.TrimSpace(options.Duplex)) {
	case "long-edge", "long", "duplex", "duplexlong":
		args = append(args, "-o", "sides=two-sided-long-edge")
	case "short-edge", "short", "duplexshort":
		args = append(args, "-o", "sides=two-sided-short-edge")
	case "simplex", "one-sided":
		args = append(args, "-o", "sides=one-sided")
	}

	switch strings.ToLower(strings.TrimSpace(options.ColorMode)) {
	case "color":
		args = append(args, "-o", "ColorModel=RGB")
	case "mono", "monochrome", "grayscale", "gray":
		args = append(args, "-o", "ColorModel=Gray")
	}

	if options.Paper != "" {
		args = append(args, "-o", fmt.Sprintf("media=%s", options.Paper))
	}

	switch strings.ToLower(strings.TrimSpace(options.Scale)) {
	case "noscale", "none", "actual":
		args = append(args, "-o", "scaling=100")
	case "shrink":
		// Default CUPS behavior already shrinks to fit if needed
	case "fit":
		args = append(args, "-o", "fit-to-page")
	}

	switch strings.ToLower(strings.TrimSpace(options.Orientation)) {
	case "portrait":
		args = append(args, "-o", "orientation-requested=3")
	case "landscape":
		args = append(args, "-o", "orientation-requested=4")
	}

	if options.TrayBin != "" {
		args = append(args, "-o", fmt.Sprintf("InputSlot=%s", options.TrayBin))
	}

	args = append(args, filePath)

	cmd := exec.Command("lp", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("lp error: %v, output: %s", err, string(out))
	}
	return nil
}
