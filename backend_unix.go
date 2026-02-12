//go:build !windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func (b *Bridge) getPrintersPlatform() ([]PrinterInfo, error) {
	defaultName := ""
	if out, err := exec.Command("lpstat", "-d").Output(); err == nil {
		line := strings.TrimSpace(string(out))
		if idx := strings.LastIndex(line, ":"); idx >= 0 {
			defaultName = strings.TrimSpace(line[idx+1:])
		}
	}

	cmd := exec.Command("sh", "-c", "lpstat -a | cut -d ' ' -f 1")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	var printers []PrinterInfo
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			printers = append(printers, PrinterInfo{Name: trimmed, IsDefault: trimmed == defaultName})
		}
	}
	return printers, nil
}

func (b *Bridge) getPrinterCapabilitiesPlatform(printerName string) (map[string]interface{}, error) {
	printerName = strings.TrimSpace(printerName)
	if printerName == "" {
		return nil, fmt.Errorf("printer name is empty")
	}

	cmd := exec.Command("lpoptions", "-p", printerName, "-l")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("lpoptions error: %v, output: %s", err, string(output))
	}

	options := map[string]map[string]interface{}{}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		optName := left
		if idx := strings.Index(left, "/"); idx >= 0 {
			optName = strings.TrimSpace(left[:idx])
		}

		values := []string{}
		defaultValue := ""
		for _, token := range strings.Fields(right) {
			if strings.HasPrefix(token, "*") {
				clean := strings.TrimPrefix(token, "*")
				defaultValue = clean
				values = append(values, clean)
				continue
			}
			values = append(values, token)
		}

		options[optName] = map[string]interface{}{
			"values":  values,
			"default": defaultValue,
			"raw":     right,
		}
	}

	return map[string]interface{}{
		"printer": printerName,
		"options": options,
	}, nil
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
