//go:build windows

package main

import "golang.org/x/sys/windows/registry"

func getDeviceID() (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()

	id, _, err := key.GetStringValue("MachineGuid")
	if err != nil {
		return "", err
	}
	return id, nil
}
