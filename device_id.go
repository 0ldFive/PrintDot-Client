package main

import "strings"

func getNormalizedDeviceID() string {
	id, err := getDeviceID()
	if err != nil {
		return ""
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}

	if strings.HasPrefix(id, "{") && strings.HasSuffix(id, "}") {
		id = strings.TrimSpace(id[1 : len(id)-1])
	}

	clean := strings.ReplaceAll(id, "-", "")
	clean = strings.ReplaceAll(clean, " ", "")
	clean = strings.ReplaceAll(clean, "\t", "")
	clean = strings.ReplaceAll(clean, "\n", "")
	clean = strings.ReplaceAll(clean, "\r", "")

	if len(clean) == 32 {
		id = clean[0:8] + "-" + clean[8:12] + "-" + clean[12:16] + "-" + clean[16:20] + "-" + clean[20:32]
	}

	return strings.ToUpper(id)
}
