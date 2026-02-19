//go:build !windows && !darwin && !linux

package main

import "os"

func getDeviceID() (string, error) {
	return os.Hostname()
}
