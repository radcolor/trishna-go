package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Monitor struct {
	baseURL         string
	configuredModel string
	httpClient      *http.Client
}

type Status struct {
	Available       bool
	Version         string
	ConfiguredModel string
	Error           string
	LoadedModels    []LoadedModel
}

type LoadedModel struct {
	Name          string
	SizeVRAM      int64
	ParameterSize string
	Quantization  string
	ContextLength int
}

type psResponse struct {
	Models []struct {
		Name          string `json:"name"`
		SizeVRAM      int64  `json:"size_vram"`
		ContextLength int    `json:"context_length"`
		Details       struct {
			ParameterSize     string `json:"parameter_size"`
			QuantizationLevel string `json:"quantization_level"`
		} `json:"details"`
	} `json:"models"`
}

type versionResponse struct {
	Version string `json:"version"`
}

func NewMonitor(baseURL, configuredModel string) Monitor {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return Monitor{
		baseURL:         baseURL,
		configuredModel: strings.TrimSpace(configuredModel),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (m Monitor) Snapshot(ctx context.Context) Status {
	status := Status{
		ConfiguredModel: m.configuredModel,
	}

	version, err := m.fetchVersion(ctx)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Available = true
	status.Version = version

	models, err := m.fetchLoadedModels(ctx)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.LoadedModels = models
	return status
}

func (m Monitor) fetchVersion(ctx context.Context) (string, error) {
	body, err := m.get(ctx, "/api/version")
	if err != nil {
		return "", err
	}
	var parsed versionResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decode ollama version: %w", err)
	}
	return strings.TrimSpace(parsed.Version), nil
}

func (m Monitor) fetchLoadedModels(ctx context.Context) ([]LoadedModel, error) {
	body, err := m.get(ctx, "/api/ps")
	if err != nil {
		return nil, err
	}
	var parsed psResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode ollama ps: %w", err)
	}

	models := make([]LoadedModel, 0, len(parsed.Models))
	for _, model := range parsed.Models {
		models = append(models, LoadedModel{
			Name:          model.Name,
			SizeVRAM:      model.SizeVRAM,
			ParameterSize: model.Details.ParameterSize,
			Quantization:  model.Details.QuantizationLevel,
			ContextLength: model.ContextLength,
		})
	}
	return models, nil
}

func (m Monitor) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create ollama request: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read ollama response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}
