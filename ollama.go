package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

//go:embed prompts/docx-system.md
var docxSystemPrompt string

//go:embed prompts/docx-user.md
var docxUserPromptTemplate string

const defaultOllamaURL = "http://localhost:11434"

type fieldUpdate struct {
	FieldName string `json:"fieldName"`
	Value     any    `json:"value"`
}

type ollamaClient struct {
	baseURL string
	model   string
	http    *http.Client
}

func newOllamaClient() *ollamaClient {
	return &ollamaClient{
		baseURL: strings.TrimRight(env("OLLAMA_URL", defaultOllamaURL), "/"),
		model:   env("OLLAMA_MODEL", "llama3.1"),
		http: &http.Client{
			Timeout: 2 * time.Minute,
		},
	}
}

func (c *ollamaClient) mapDOCXToFields(ctx context.Context, contentItemJSON string, docxXML string) ([]fieldUpdate, error) {
	userPrompt := strings.ReplaceAll(docxUserPromptTemplate, "{{CONTENT_ITEM_JSON}}", cleanPromptFenceContent(contentItemJSON))
	userPrompt = strings.ReplaceAll(userPrompt, "{{DOCX_XML}}", cleanPromptFenceContent(docxXML))

	reqBody := map[string]any{
		"model":  c.model,
		"stream": false,
		"format": "json",
		"messages": []map[string]string{
			{"role": "system", "content": docxSystemPrompt},
			{"role": "user", "content": userPrompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("encode ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call Ollama at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var responseBody bytes.Buffer
		_, _ = responseBody.ReadFrom(resp.Body)
		return nil, fmt.Errorf("Ollama returned %s: %s", resp.Status, strings.TrimSpace(responseBody.String()))
	}

	var ollamaResp struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decode ollama response: %w", err)
	}
	if ollamaResp.Error != "" {
		return nil, fmt.Errorf("Ollama error: %s", ollamaResp.Error)
	}

	fmt.Printf("AI output:\n%s\n", ollamaResp.Message.Content)

	updates, err := parseFieldUpdates(ollamaResp.Message.Content)
	if err != nil {
		return nil, err
	}
	return updates, nil
}

func parseFieldUpdates(content string) ([]fieldUpdate, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var updates []fieldUpdate
	if err := json.Unmarshal([]byte(content), &updates); err != nil {
		return nil, fmt.Errorf("AI response was not a JSON field update array: %w", err)
	}

	filtered := updates[:0]
	for _, update := range updates {
		update.FieldName = strings.TrimSpace(update.FieldName)
		if update.FieldName == "" || update.Value == nil {
			continue
		}
		filtered = append(filtered, update)
	}
	return filtered, nil
}

func cleanPromptFenceContent(value string) string {
	return strings.ReplaceAll(value, "```", "`\u200b``")
}
