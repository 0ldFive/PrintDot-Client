package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

//go:embed locales/*.json
var localeFS embed.FS

var (
	translations map[string]interface{}
	i18nMu       sync.RWMutex
)

// LoadLocales loads the locale file based on the language code
func LoadLocales(lang string) error {
	i18nMu.Lock()
	defer i18nMu.Unlock()

	// Map generic codes to file names
	filename := "locales/en.json"
	if lang == "zh-CN" || lang == "zh" {
		filename = "locales/zh.json"
	}

	content, err := localeFS.ReadFile(filename)
	if err != nil {
		// Fallback to en if specific locale fails
		if filename != "locales/en.json" {
			content, err = localeFS.ReadFile("locales/en.json")
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	var data map[string]interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		return err
	}

	translations = data
	return nil
}

// T translates a key. Nested keys can be accessed with dot notation (e.g., "menu.title")
func T(key string, args ...interface{}) string {
	i18nMu.RLock()
	defer i18nMu.RUnlock()

	if translations == nil {
		return key
	}

	keys := strings.Split(key, ".")
	var val interface{} = translations

	for _, k := range keys {
		if m, ok := val.(map[string]interface{}); ok {
			if v, exists := m[k]; exists {
				val = v
			} else {
				return key
			}
		} else {
			return key
		}
	}

	if str, ok := val.(string); ok {
		if len(args) > 0 {
			return fmt.Sprintf(str, args...)
		}
		return str
	}

	return key
}
