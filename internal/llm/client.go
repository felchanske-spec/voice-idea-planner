package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/felchanske-spec/voice-idea-planner/internal/domain"
)

type GeminiClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewGeminiClient(apiKey string) *GeminiClient {
	return &GeminiClient{
		apiKey: apiKey,
		model:  "gemini-3.5-flash",
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

type BlobData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type Part struct {
	Text       string    `json:"text,omitempty"`
	InlineData *BlobData `json:"inlineData,omitempty"`
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
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", c.model)

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

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("api returned bad status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from gemini api")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

func (c *GeminiClient) Transcription(ctx context.Context, audioBytes []byte) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", c.model)

	base64Audio := base64.StdEncoding.EncodeToString(audioBytes)

	systemPrompt := "Ты - профессиональный и строгий стенографист. Твоя задача — прослушать аудиозапись и перевести её в текст на русском языке ДОСЛОВНО. " +
		"Запрещено пересказывать, делать выводы, исправлять мысли или добавлять что-то от себя. " +
		"ПРАВИЛА ОБРАБОТКИ НЕРАЗБОРЧИВЫХ СЛОВ:\n" +
		"1. Если в аудиозаписи есть неразборчивый отрывок длиной НЕ БОЛЕЕ 1-2 слов, попытайся восстановить их по общему ИТ-смыслу текста.\n" +
		"2. Если неразборчивый отрывок составляет 3 И БОЛЕЕ слов, или аудиозапись полностью состоит из шума — не пытайся ничего придумывать. В этом случае твой ответ должен состоять строго из одного слова: ERROR_TRANSCRIPTION_FAILED"

	reqBody := GeminiRequest{
		Contents: []Content{
			{
				Parts: []Part{
					{Text: systemPrompt},
					{
						InlineData: &BlobData{
							MimeType: "audio/ogg",
							Data:     base64Audio,
						},
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal transcription request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create transcription request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("transcription http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("transcription api returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read transcription response: %w", err)
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal transcription response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty transcription response from gemini")
	}

	resultText := geminiResp.Candidates[0].Content.Parts[0].Text

	if resultText == "ERROR_TRANSCRIPTION_FAILED" {
		return "", domain.ErrEmptyVoiceText
	}

	return resultText, nil
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
