//go:build windows

package main

import (
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

func (b *Bridge) printPDFPlatform(printerName, filePath string, options PrintOptions) error {
	sumatraPath, err := findSumatraPDF()
	if err != nil {
		return err
	}

	settings := buildSumatraPrintSettings(options)
	if options.PageRange != "" && options.PrintSettings == "" {
		b.Log("PageRange is not applied for SumatraPDF unless printSettings is provided")
	}

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
		return options.PrintSettings
	}

	var settings []string

	switch strings.ToLower(strings.TrimSpace(options.Scale)) {
	case "fit":
		settings = append(settings, "fit")
	case "shrink":
		settings = append(settings, "shrink")
	case "none", "actual":
		settings = append(settings, "none")
	}

	switch strings.ToLower(strings.TrimSpace(options.Duplex)) {
	case "long-edge", "long", "duplex", "duplexlong":
		settings = append(settings, "duplex")
	case "short-edge", "short", "duplexshort":
		settings = append(settings, "duplexshort")
	}

	switch strings.ToLower(strings.TrimSpace(options.ColorMode)) {
	case "mono", "monochrome", "grayscale", "gray":
		settings = append(settings, "mono")
	}

	if options.Paper != "" {
		settings = append(settings, fmt.Sprintf("paper=%s", options.Paper))
	}

	return strings.Join(settings, ",")
}
