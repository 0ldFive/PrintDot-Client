//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procMultiByteToWideChar = kernel32.NewProc("MultiByteToWideChar")
)

// ansiToUtf8 converts ANSI (CP_ACP) bytes to UTF-8 string
func ansiToUtf8(b []byte) (string, error) {
	if len(b) == 0 {
		return "", nil
	}

	// CP_ACP = 0
	// 1. Get required length
	ret, _, _ := procMultiByteToWideChar.Call(
		0, // CP_ACP
		0,
		uintptr(unsafe.Pointer(&b[0])),
		uintptr(len(b)),
		0,
		0,
	)
	if ret == 0 {
		return "", fmt.Errorf("MultiByteToWideChar failed")
	}

	// 2. Allocate buffer
	utf16buf := make([]uint16, ret)

	// 3. Convert
	ret, _, _ = procMultiByteToWideChar.Call(
		0,
		0,
		uintptr(unsafe.Pointer(&b[0])),
		uintptr(len(b)),
		uintptr(unsafe.Pointer(&utf16buf[0])),
		ret,
	)
	if ret == 0 {
		return "", fmt.Errorf("MultiByteToWideChar failed")
	}

	return syscall.UTF16ToString(utf16buf), nil
}

// decodeCmdOutput handles WMIC encoding quirks (UTF-16LE BOM or ANSI)
func decodeCmdOutput(output []byte) (string, error) {
	if len(output) >= 2 && output[0] == 0xFF && output[1] == 0xFE {
		// UTF-16LE BOM detected
		// Skip BOM
		raw16 := output[2:]
		// Make sure even number of bytes
		if len(raw16)%2 != 0 {
			raw16 = append(raw16, 0)
		}
		u16s := make([]uint16, len(raw16)/2)
		for i := 0; i < len(u16s); i++ {
			u16s[i] = uint16(raw16[i*2]) | uint16(raw16[i*2+1])<<8
		}
		return syscall.UTF16ToString(u16s), nil
	}

	// Assume ANSI (e.g. GBK on Chinese Windows)
	return ansiToUtf8(output)
}

func (b *Bridge) getPrintersPlatform() ([]string, error) {
	// Use WMIC to get printer names
	// wmic printer get name
	cmd := exec.Command("wmic", "printer", "get", "name")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	decodedStr, err := decodeCmdOutput(output)
	if err != nil {
		// Fallback
		decodedStr = string(output)
	}

	// WMIC output can be messy with CR/LF
	lines := strings.Split(decodedStr, "\n")
	var printers []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip header "Name" and empty lines
		if trimmed != "" && trimmed != "Name" {
			printers = append(printers, trimmed)
		}
	}
	return printers, nil
}

func (b *Bridge) getPrinterCapabilitiesPlatform(printerName string) (map[string]interface{}, error) {
	printerName = strings.TrimSpace(printerName)
	if printerName == "" {
		return nil, fmt.Errorf("printer name is empty")
	}

	escaped := strings.ReplaceAll(printerName, "'", "''")
	ps := fmt.Sprintf(`$p = Get-CimInstance Win32_Printer -Filter "Name='%s'"; `+
		`$c = Get-CimInstance Win32_PrinterConfiguration -Filter "Name='%s'"; `+
		`if ($null -eq $p) { throw "Printer not found" }; `+
		`[pscustomobject]@{ `+
		`name = $p.Name; `+
		`defaultPaperSize = $p.DefaultPaperSize; `+
		`printerPaperNames = $p.PrinterPaperNames; `+
		`paperSizes = $p.PaperSizes; `+
		`colorSupported = $p.ColorSupported; `+
		`duplexSupported = $p.DuplexSupported; `+
		`driverName = $p.DriverName; `+
		`portName = $p.PortName; `+
		`printProcessor = $p.PrintProcessor; `+
		`deviceId = $p.DeviceID; `+
		`config = @{ `+
		`paperSize = $c.PaperSize; `+
		`orientation = $c.Orientation; `+
		`color = $c.Color; `+
		`duplex = $c.Duplex; `+
		`copies = $c.Copies `+
		`} `+
		`} | ConvertTo-Json -Depth 4`, escaped, escaped)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("get printer capabilities failed: %v", err)
	}

	decoded, err := decodeCmdOutput(output)
	if err != nil {
		decoded = string(output)
	}

	var caps map[string]interface{}
	if err := json.Unmarshal([]byte(decoded), &caps); err != nil {
		return nil, fmt.Errorf("parse printer capabilities failed: %v", err)
	}

	return caps, nil
}

func (b *Bridge) printPDFPlatform(printerName, filePath string, options PrintOptions) error {
	sumatraPath, err := findSumatraPDF()
	if err != nil {
		return err
	}

	settings := buildSumatraPrintSettings(options)

	args := []string{"-print-to", printerName}
	if settings != "" {
		args = append(args, "-print-settings", settings)
	}
	args = append(args, filePath)

	cmd := exec.Command(sumatraPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sumatra print failed: %v, output: %s", err, strings.TrimSpace(string(output)))
	}

	return nil
}

func findSumatraPDF() (string, error) {
	if envPath := strings.TrimSpace(os.Getenv("SUMATRAPDF_PATH")); envPath != "" {
		if fileExists(envPath) {
			return envPath, nil
		}
	}

	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "SumatraPDF.exe")
		if fileExists(candidate) {
			return candidate, nil
		}
	}

	if path, err := exec.LookPath("SumatraPDF.exe"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("SumatraPDF"); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("SumatraPDF.exe not found. Place it next to the app, add it to PATH, or set SUMATRAPDF_PATH")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func buildSumatraPrintSettings(options PrintOptions) string {
	if options.PrintSettings != "" {
		return mergeSumatraSettings(options.PrintSettings, options.Orientation, options.Paper)
	}

	var settings []string

	if options.PageRange != "" {
		settings = append(settings, options.PageRange)
	}

	switch strings.ToLower(strings.TrimSpace(options.PageSet)) {
	case "even":
		settings = append(settings, "even")
	case "odd":
		settings = append(settings, "odd")
	}

	switch strings.ToLower(strings.TrimSpace(options.Scale)) {
	case "fit":
		settings = append(settings, "fit")
	case "shrink":
		settings = append(settings, "shrink")
	case "none", "actual", "noscale":
		settings = append(settings, "noscale")
	}

	switch strings.ToLower(strings.TrimSpace(options.Orientation)) {
	case "portrait":
		settings = append(settings, "portrait")
	case "landscape":
		settings = append(settings, "landscape")
	}

	switch strings.ToLower(strings.TrimSpace(options.Duplex)) {
	case "simplex", "one-sided":
		settings = append(settings, "simplex")
	case "long-edge", "long", "duplex", "duplexlong":
		settings = append(settings, "duplex")
	case "short-edge", "short", "duplexshort":
		settings = append(settings, "duplexshort")
	}

	switch strings.ToLower(strings.TrimSpace(options.ColorMode)) {
	case "color":
		settings = append(settings, "color")
	case "mono", "monochrome", "grayscale", "gray":
		settings = append(settings, "monochrome")
	}

	if options.Paper != "" {
		settings = append(settings, fmt.Sprintf("paper=%s", options.Paper))
	}

	if options.TrayBin != "" {
		settings = append(settings, fmt.Sprintf("bin=%s", options.TrayBin))
	}

	if options.Copies > 1 {
		settings = append(settings, fmt.Sprintf("%dx", options.Copies))
	}

	return strings.Join(settings, ",")
}

func mergeSumatraSettings(settings string, orientation string, paper string) string {
	parts := strings.Split(settings, ",")
	result := make([]string, 0, len(parts)+1)
	seenPaper := false

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if lower == "portrait" || lower == "landscape" {
			continue
		}
		if strings.HasPrefix(lower, "paper=") {
			seenPaper = true
		}
		result = append(result, trimmed)
	}

	switch strings.ToLower(strings.TrimSpace(orientation)) {
	case "portrait", "landscape":
		result = append(result, strings.ToLower(strings.TrimSpace(orientation)))
	}

	if strings.TrimSpace(paper) != "" && !seenPaper {
		result = append(result, fmt.Sprintf("paper=%s", paper))
	}

	return strings.Join(result, ",")
}
