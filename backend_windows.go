//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	shell32                 = syscall.NewLazyDLL("shell32.dll")
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procShellExecuteW       = shell32.NewProc("ShellExecuteW")
	procMultiByteToWideChar = kernel32.NewProc("MultiByteToWideChar")
)

const swHide = 0

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

func shellExecute(hwnd uintptr, operation, file, parameters, directory string, showCmd int) error {
	op, _ := syscall.UTF16PtrFromString(operation)
	f, _ := syscall.UTF16PtrFromString(file)
	p, _ := syscall.UTF16PtrFromString(parameters)
	d, _ := syscall.UTF16PtrFromString(directory)

	ret, _, _ := procShellExecuteW.Call(
		hwnd,
		uintptr(unsafe.Pointer(op)),
		uintptr(unsafe.Pointer(f)),
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(d)),
		uintptr(showCmd),
	)

	// ShellExecute returns a value greater than 32 if successful
	if ret <= 32 {
		return fmt.Errorf("ShellExecute failed with code %d", ret)
	}
	return nil
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

func (b *Bridge) printPDFPlatform(printerName, filePath string) error {
	// 1. Get current default printer
	// wmic printer where default='true' get name
	cmd := exec.Command("wmic", "printer", "where", "default='true'", "get", "name")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("get default printer failed: %v", err)
	}

	decodedStr, err := decodeCmdOutput(out)
	if err != nil {
		decodedStr = string(out)
	}

	lines := strings.Split(decodedStr, "\n")
	var defaultPrinter string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && trimmed != "Name" {
			defaultPrinter = trimmed
			break
		}
	}

	// 2. If target is different, set it as default
	needRestore := false
	if defaultPrinter != printerName {
		b.Log(fmt.Sprintf("Switching default printer from '%s' to '%s' temporarily", defaultPrinter, printerName))
		// RUNDLL32 PRINTUI.DLL,PrintUIEntry /y /n "Printer Name"
		cmd := exec.Command("rundll32", "printui.dll,PrintUIEntry", "/y", "/n", printerName)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("set default printer failed: %v", err)
		}
		needRestore = true
	}

	// 3. Print using ShellExecute
	// Strategy:
	// 1. Try "printto" verb first (specific printer).
	// 2. If "printto" fails with SE_ERR_NOASSOC (31), fallback to "print" verb (default printer).
	//    Since we already switched the default printer, "print" should work and target the correct printer.

	// Try printto with printer name as parameter
	// Some readers expect the printer name in quotes
	printToParams := fmt.Sprintf("\"%s\"", printerName)
	err = shellExecute(0, "printto", filePath, printToParams, "", swHide)

	if err != nil && strings.Contains(err.Error(), "code 31") {
		b.Log("verb 'printto' not supported (code 31), falling back to 'print' verb...")
		// Fallback to "print" which uses default printer (which we just set)
		err = shellExecute(0, "print", filePath, "", "", swHide)
	}

	if err != nil {
		// Enhance error message if it's still 31
		if strings.Contains(err.Error(), "code 31") {
			err = fmt.Errorf("printing failed: No PDF reader installed or associated with 'print'/'printto' verbs. Please install Adobe Reader, Foxit Reader, or similar. (System error: %v)", err)
		}
	}

	// 4. Restore default if needed
	if needRestore {
		// Wait for spooling - critical for avoiding race conditions
		time.Sleep(3 * time.Second)

		cmd := exec.Command("rundll32", "printui.dll,PrintUIEntry", "/y", "/n", defaultPrinter)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if rErr := cmd.Run(); rErr != nil {
			b.Log(fmt.Sprintf("Warning: Failed to restore default printer: %v", rErr))
		} else {
			b.Log("Restored default printer")
		}
	} else {
		// Even if we didn't switch, give it a moment to ensure command execution
		time.Sleep(1 * time.Second)
	}

	return err
}
