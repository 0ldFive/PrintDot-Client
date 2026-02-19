//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

func setAutoStart(enabled bool) {
	exe, err := os.Executable()
	if err != nil {
		return
	}

	if enabled {
		cmd := exec.Command("reg", "add", "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run", "/v", "PrintDotClient", "/t", "REG_SZ", "/d", exe, "/f")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = cmd.Run()
		return
	}

	cmd := exec.Command("reg", "delete", "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run", "/v", "PrintDotClient", "/f")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Run()
}
