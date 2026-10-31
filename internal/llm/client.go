package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type GeminiClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

type Part struct {
	Text string `json:"text"`
}

type Content struct {
	Parts []Part `json:"parts"`
}

type GeminiRequest struct {
	Contents []Content `json:"contents"`
}

type ResponseContent struct {
	Parts []Part `json:"parts"`
}

type Candidate struct {
	Content ResponseContent `json:"content"`
}

type GeminiResponse struct {
	Candidates []Candidate `json:"candidates"`
}

func (c *GeminiClient) query(ctx context.Context, prompt string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", c.model, c.apiKey)

	reqBody := GeminiRequest{
		Contents: []Content{
			{
				Parts: []Part{{Text: prompt}},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "applicaetion/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("api retured bad tatus %d: %s", resp.StatusCode, string(bodyBytes))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to unmarhsal response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from gemini api")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

func (c *GeminiClient) TitleGen(ctx context.Context, rawText string) (string, error) {
	prompt := fmt.Sprintf(
		"Ты - опытный ИТ-продукт-менеджер. Проанализируй текст идеи и верни ровно ОДНУ короткую строку (до 5-7 слов) — понятное и звучное название для этого проекта на русском языке. Не пиши никаких пояснений, введений или кавычек. Только само название. Текст идеи: %s",
		rawText,
	)
	return c.query(ctx, prompt)
}

func (c *GeminiClient) MermaidGen(ctx context.Context, rawText string) (string, error) {
	prompt := fmt.Sprintf(
		"Ты - системный архитектор. Проанализируй идею и составь для неё архитектурную диаграмму или флоу-чарт на языке Mermaid.js (используй синтаксис 'graph TD'). Верни ТОЛЬКО сырой текст разметки Mermaid. Не оборачивай код в кавычки типа ```mermaid и не пиши никакого текста вокруг. Начни ответ прямо со слова graph. Текст идеи: %s",
		rawText,
	)
	return c.query(ctx, prompt)
}

func (c *GeminiClient) GenerateSection(ctx context.Context, rawText string, sectionType string) (string, error) {
	var instruction string

	switch sectionType {
	case "mvp":
		instruction = "Опиши минимально жизнеспособный продукт (MVP Scope) для этой идеи. Выдели ключевой функционал, который нужно сделать в первую очередь, а что можно отложить. Ответ напиши в виде списка с маркерами на русском языке."
	case "stack":
		instruction = "Рекомендуй оптимальный технологический стек для реализации этого проекта (языки, базы данных, библиотеки, хостинг). Кратко обоснуй выбор каждого компонента. Ответ отформатируй в виде списка на русском языке."
	case "risk":
		instruction = "Проанализируй потенциальные технические и продуктовые риски этого проекта (например, проблемы с API, масштабируемость, безопасность) и предложи базовые способы их минимизации. Ответ напиши в виде списка на русском языке."
	default:
		return "", fmt.Errorf("unknown section type: %s", sectionType)
	}

	prompt := fmt.Sprintf(
		"Ты — ведущий инженер и технический консультант. %s\n\nИсходный текст идеи: %s",
		instruction, rawText,
	)
	return c.query(ctx, prompt)
}
