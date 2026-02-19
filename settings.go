package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cloudfoundry/jibber_jabber"
)

type AppSettings struct {
	Language          string `json:"language"`
	AutoStart         bool   `json:"autoStart"`
	RemoteAutoConnect bool   `json:"remoteAutoConnect"`
	RemoteServer      string `json:"remoteServer"`
	RemoteAuthURL     string `json:"remoteAuthUrl"`
	RemoteWsURL       string `json:"remoteWsUrl"`
	RemoteUser        string `json:"remoteUser"`
	RemotePassword    string `json:"remotePassword"`
	RemoteClientID    string `json:"remoteClientId"`
	RemoteSecretKey   string `json:"remoteSecretKey"`
	RemoteClientName  string `json:"remoteClientName"`
	// Window State
	WindowWidth  int  `json:"windowWidth"`
	WindowHeight int  `json:"windowHeight"`
	WindowX      int  `json:"windowX"`
	WindowY      int  `json:"windowY"`
	Maximized    bool `json:"maximized"`
}

type SettingsManager struct {
	settings AppSettings
	mu       sync.Mutex
	filePath string
}

func NewSettingsManager() *SettingsManager {
	appConfigDir, err := dataDirPath()
	if err != nil {
		configDir, _ := os.UserConfigDir()
		appConfigDir = filepath.Join(configDir, "PrintDot")
	}
	os.MkdirAll(appConfigDir, 0755)

	// Detect language
	defaultLang := "en-US"
	userLang, err := jibber_jabber.DetectLanguage()
	if err == nil && strings.ToLower(userLang) == "zh" {
		defaultLang = "zh-CN"
	}

	sm := &SettingsManager{
		filePath: filepath.Join(appConfigDir, "settings.json"),
		settings: AppSettings{
			Language:          defaultLang,
			AutoStart:         false,
			RemoteAutoConnect: true,
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
		var raw map[string]interface{}
		if json.Unmarshal(data, &raw) == nil {
			if _, ok := raw["remoteAutoConnect"]; !ok {
				sm.settings.RemoteAutoConnect = true
			}
		}
	}

	if sm.settings.RemoteClientID == "" && sm.settings.RemoteUser != "" {
		sm.settings.RemoteClientID = sm.settings.RemoteUser
	}
	if sm.settings.RemoteSecretKey == "" && sm.settings.RemotePassword != "" {
		sm.settings.RemoteSecretKey = sm.settings.RemotePassword
	}

	if sm.settings.RemoteAuthURL == "" && sm.settings.RemoteWsURL == "" && sm.settings.RemoteServer != "" {
		if loginURL, wsURL, err := buildRemoteURLs(sm.settings.RemoteServer); err == nil {
			sm.settings.RemoteAuthURL = loginURL.String()
			sm.settings.RemoteWsURL = wsURL.String()
		}
	}

	applyDefaultClientIdentity(&sm.settings)
}

func (sm *SettingsManager) Save(settings AppSettings) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if settings.RemoteClientID == "" && settings.RemoteUser != "" {
		settings.RemoteClientID = settings.RemoteUser
	}
	if settings.RemoteUser == "" && settings.RemoteClientID != "" {
		settings.RemoteUser = settings.RemoteClientID
	}
	if settings.RemoteSecretKey == "" && settings.RemotePassword != "" {
		settings.RemoteSecretKey = settings.RemotePassword
	}
	if settings.RemotePassword == "" && settings.RemoteSecretKey != "" {
		settings.RemotePassword = settings.RemoteSecretKey
	}

	if settings.RemoteAuthURL == "" && settings.RemoteWsURL == "" && settings.RemoteServer != "" {
		if loginURL, wsURL, err := buildRemoteURLs(settings.RemoteServer); err == nil {
			settings.RemoteAuthURL = loginURL.String()
			settings.RemoteWsURL = wsURL.String()
		}
	}

	applyDefaultClientIdentity(&settings)

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

func applyDefaultClientIdentity(settings *AppSettings) {
	id, name := defaultClientIdentity()
	if id != "" {
		settings.RemoteClientID = id
	}
	if settings.RemoteClientName == "" {
		settings.RemoteClientName = name
	}
}

func defaultClientIdentity() (string, string) {
	name := strings.TrimSpace(os.Getenv("COMPUTERNAME"))
	if name == "" {
		if host, err := os.Hostname(); err == nil {
			name = strings.TrimSpace(host)
		}
	}
	if name == "" {
		name = "PrintDot"
	}

	id := getNormalizedDeviceID()
	if id == "" {
		id = strings.ToLower(name)
		id = strings.ReplaceAll(id, " ", "-")
		id = strings.ReplaceAll(id, "_", "-")
		id = strings.ReplaceAll(id, "/", "-")
	}

	return id, name
}
