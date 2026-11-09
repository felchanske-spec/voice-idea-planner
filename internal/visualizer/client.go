package visualizer

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type MermaidVisualizer struct {
	baseURL    string
	httpClient *http.Client
}

func NewMermaidVisualizer() *MermaidVisualizer {
	return &MermaidVisualizer{
		baseURL: "https://mermaid.ink/img",
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (v *MermaidVisualizer) GenerateDiagram(ctx context.Context, mermaidCode string) ([]byte, error) {
	if strings.TrimSpace(mermaidCode) == "" {
		return nil, fmt.Errorf("mermaid code is empty")
	}

	encodedCode := base64.URLEncoding.EncodeToString([]byte(mermaidCode))

	encodedCode = strings.TrimRight(encodedCode, "=")

	finalURL := fmt.Sprintf("%s/%s", v.baseURL, encodedCode)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, finalURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mermaid.ink http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mermaid.ink API returned bad status: %d", resp.StatusCode)
	}

	imageBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image response body: %w", err)
	}

	return imageBytes, nil
}
