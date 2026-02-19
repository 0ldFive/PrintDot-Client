//go:build !windows

package main

func setAutoStart(enabled bool) {
	_ = enabled
}
