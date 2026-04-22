// Package main provides a command-line tool to fetch models from Kimi
// and generate a configuration file for the provider.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"charm.land/catwalk/pkg/catwalk"
)

// ModelsResponse represents the OpenAI-compatible models API response.
type ModelsResponse struct {
	Object string      `json:"object"`
	Data   []KimiModel `json:"data"`
}

// KimiModel represents a model from the Kimi API.
type KimiModel struct {
	ID                string `json:"id"`
	Object            string `json:"object"`
	Created           int64  `json:"created"`
	DisplayName       string `json:"display_name"`
	ContextLength     int64  `json:"context_length"`
	SupportsReasoning bool   `json:"supports_reasoning"`
	SupportsImageIn   bool   `json:"supports_image_in"`
	SupportsVideoIn   bool   `json:"supports_video_in"`
}

func fetchKimiModels() (*ModelsResponse, error) {
	apiKey := os.Getenv("KIMI_CODING_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("KIMI_CODING_API_KEY environment variable is not set")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(
		context.Background(),
		"GET",
		"https://api.kimi.com/coding/v1/models",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Crush-Client/1.0")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err //nolint:wrapcheck
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, body)
	}

	_ = os.MkdirAll("tmp", 0o700)
	_ = os.WriteFile("tmp/kimi-response.json", body, 0o600)

	var mr ModelsResponse
	if err := json.Unmarshal(body, &mr); err != nil {
		return nil, err //nolint:wrapcheck
	}
	return &mr, nil
}

func main() {
	modelsResp, err := fetchKimiModels()
	if err != nil {
		log.Fatal("Error fetching Kimi models:", err)
	}

	provider := catwalk.Provider{
		Name:                "Kimi Coding",
		ID:                  catwalk.InferenceKimiCoding,
		APIKey:              "$KIMI_CODING_API_KEY",
		APIEndpoint:         "https://api.kimi.com/coding",
		Type:                catwalk.TypeOpenAICompat,
		DefaultLargeModelID: "kimi-for-coding",
		DefaultSmallModelID: "kimi-for-coding",
	}

	for _, model := range modelsResp.Data {
		name := model.DisplayName
		if name == "" {
			name = model.ID
		}

		defaultMaxTokens := min(model.ContextLength/8, 32768)

		m := catwalk.Model{
			ID:                 model.ID,
			Name:               name,
			CostPer1MIn:        0,
			CostPer1MOut:       0,
			CostPer1MInCached:  0,
			CostPer1MOutCached: 0,
			ContextWindow:      model.ContextLength,
			DefaultMaxTokens:   defaultMaxTokens,
			CanReason:          model.SupportsReasoning,
			SupportsImages:     model.SupportsImageIn || model.SupportsVideoIn,
		}

		if model.SupportsReasoning {
			m.Features = []catwalk.ModelSpecialFeatures{catwalk.KimiThinking}
		}

		provider.Models = append(provider.Models, m)
		fmt.Printf("Added model %s (%s)\n", model.ID, name)
	}

	data, err := json.MarshalIndent(provider, "", "  ")
	if err != nil {
		log.Fatal("Error marshaling Kimi provider:", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile("internal/providers/configs/kimi.json", data, 0o600); err != nil {
		log.Fatal("Error writing Kimi provider config:", err)
	}

	fmt.Printf("Generated kimi.json with %d models\n", len(provider.Models))
}
