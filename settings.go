package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

type AppSettings struct {
	Language       string `json:"language"`
	AutoStart      bool   `json:"autoStart"`
	RemoteServer   string `json:"remoteServer"`
	RemoteUser     string `json:"remoteUser"`
	RemotePassword string `json:"remotePassword"`
}

type SettingsManager struct {
	settings AppSettings
	mu       sync.Mutex
	filePath string
}

func NewSettingsManager() *SettingsManager {
	configDir, _ := os.UserConfigDir()
	appConfigDir := filepath.Join(configDir, "print-dot-client")
	os.MkdirAll(appConfigDir, 0755)

	sm := &SettingsManager{
		filePath: filepath.Join(appConfigDir, "settings.json"),
		settings: AppSettings{
			Language:  "zh-CN",
			AutoStart: false,
		},
	}
	sm.Load()
	return sm
}

func (sm *SettingsManager) Load() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	data, err := os.ReadFile(sm.filePath)
	if err == nil {
		json.Unmarshal(data, &sm.settings)
	}
}

func (sm *SettingsManager) Save(settings AppSettings) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.settings = settings
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	// Handle AutoStart
	exe, err := os.Executable()
	if err == nil {
		if settings.AutoStart {
			exec.Command("reg", "add", "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run", "/v", "PrintDotClient", "/t", "REG_SZ", "/d", exe, "/f").Run()
		} else {
			exec.Command("reg", "delete", "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run", "/v", "PrintDotClient", "/f").Run()
		}
	}

	return os.WriteFile(sm.filePath, data, 0644)
}

func (sm *SettingsManager) Get() AppSettings {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.settings
}
