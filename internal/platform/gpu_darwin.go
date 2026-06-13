//go:build darwin

package platform

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"sync"
	"time"
)

var (
	gpuCacheMu sync.Mutex
	gpuCache   string
	gpuCacheAt time.Time
)

func gpuName() string {
	gpuCacheMu.Lock()
	defer gpuCacheMu.Unlock()

	if gpuCache != "" && time.Since(gpuCacheAt) < time.Hour {
		return gpuCache
	}

	name := readGPUName()
	if name == "" {
		name = "Apple GPU"
	}
	gpuCache = name
	gpuCacheAt = time.Now()
	return name
}

func hostModel() string {
	out, err := exec.Command("sysctl", "-n", "hw.model").Output()
	if err != nil {
		return ""
	}
	return string(bytes.TrimSpace(out))
}

func readGPUName() string {
	out, err := exec.Command("system_profiler", "SPDisplaysDataType", "-json").Output()
	if err != nil {
		return ""
	}

	var data struct {
		SPDisplaysDataType []struct {
			SPPCI_Model string `json:"sppci_model"`
			Chipset     string `json:"_name"`
		} `json:"SPDisplaysDataType"`
	}
	if err := json.Unmarshal(out, &data); err != nil {
		return ""
	}
	for _, display := range data.SPDisplaysDataType {
		if display.SPPCI_Model != "" {
			return display.SPPCI_Model
		}
		if display.Chipset != "" {
			return display.Chipset
		}
	}
	return ""
}
