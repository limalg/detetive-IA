package game

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"detetive-ia/internal/models"
)

// AIClient defines the contract for LLM generation.
type AIClient interface {
	GenerateResponse(ctx context.Context, prompt string) (string, error)
}

// AIConfig holds settings for LLM generation.
type AIConfig struct {
	URL      string `json:"url"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	Provider string `json:"provider"` // "openai_compatible", "gemini"
}

// UniversalAIClient implements AIClient supporting OpenAI-compatible routers and Gemini.
type UniversalAIClient struct {
	mu         sync.RWMutex
	config     AIConfig
	httpClient *http.Client
}

// NewUniversalAIClient creates a configurable AI client.
func NewUniversalAIClient(cfg AIConfig) *UniversalAIClient {
	if cfg.URL == "" {
		cfg.URL = "https://router.ofertasshow.shop/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "ondegastei"
	}
	if cfg.Provider == "" {
		cfg.Provider = "openai_compatible"
	}
	return &UniversalAIClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// UpdateConfig hot-reloads the AI client configuration in a thread-safe manner.
func (u *UniversalAIClient) UpdateConfig(cfg AIConfig) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.config = cfg
}

// GetConfig returns the active AI configuration in a thread-safe manner.
func (u *UniversalAIClient) GetConfig() AIConfig {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.config
}

func (u *UniversalAIClient) GenerateResponse(ctx context.Context, prompt string) (string, error) {
	startTime := time.Now()
	traceID := "ai-" + fmt.Sprintf("%d", startTime.UnixNano())

	cfg := u.GetConfig()
	if cfg.APIKey == "" {
		return "", errors.New("ai api key is not configured")
	}

	if cfg.Provider == "gemini" {
		url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", strings.TrimRight(cfg.URL, "/"), cfg.Model, cfg.APIKey)
		reqBody := map[string]interface{}{
			"contents": []map[string]interface{}{
				{"parts": []map[string]string{{"text": prompt}}},
			},
		}
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return "", err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := u.httpClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("gemini error (status %d): %s", resp.StatusCode, string(b))
		}
		var result struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", err
		}
		if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
			return strings.TrimSpace(result.Candidates[0].Content.Parts[0].Text), nil
		}
		return "", errors.New("empty response from gemini")
	}

	// OpenAI-compatible endpoint (e.g. router.ofertasshow.shop/v1)
	baseURL := strings.TrimRight(cfg.URL, "/")
	endpoint := baseURL
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint = baseURL + "/chat/completions"
	}

	reqBody := map[string]interface{}{
		"model": cfg.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.7,
		"stream":      false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := u.httpClient.Do(req)
	durationMs := time.Since(startTime).Milliseconds()
	if err != nil {
		_ = traceID
		return "", fmt.Errorf("router api connection error: %w (duration: %dms)", err, durationMs)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("router api error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	trimmedBody := strings.TrimSpace(string(bodyBytes))

	// Handle SSE streaming chunks if router returns text/event-stream
	if strings.HasPrefix(trimmedBody, "data:") {
		var contentBuilder strings.Builder
		lines := strings.Split(trimmedBody, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "[DONE]" || payload == "" {
				continue
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(payload), &chunk); err == nil {
				if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
					contentBuilder.WriteString(chunk.Choices[0].Delta.Content)
				}
			}
		}
		if contentBuilder.Len() > 0 {
			return strings.TrimSpace(contentBuilder.String()), nil
		}
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(bodyBytes, &chatResp); err == nil && len(chatResp.Choices) > 0 && chatResp.Choices[0].Message.Content != "" {
		return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
	}

	return "", errors.New("empty choices response from router api: " + trimmedBody)
}

// GenerateFallbackResponseWithStress generates an in-character Noir response factoring in suspect stress levels and 3-tier secrets.
func GenerateFallbackResponseWithStress(suspect *models.Suspect, question string, stress int) string {
	q := strings.ToLower(question)

	// High Stress (>70%): Breakdown and confession / desperate truth (Tier 3)
	if stress > 70 {
		truth := suspect.Tier3Truth
		if truth == "" {
			truth = "Eu perdi o controle! As coisas foram longe demais naquela sala!"
		}
		if suspect.IsCulprit {
			return fmt.Sprintf("O suspeito empalidece em choque absoluto e desmorona com a voz trêmula: '%s'", truth)
		}
		return fmt.Sprintf("O suspeito entra em pânico perante o cerco policial: '%s %s'", truth, suspect.CrossSuspicion)
	}

	// Medium Stress (30%-70%): Defensive admission of secondary petty crime (Tier 2)
	if stress >= 30 {
		petty := suspect.Tier2PettyCrime
		if petty == "" {
			petty = suspect.Secret
		}
		if petty == "" {
			petty = "Eu ocultava pequenos desvios contábeis e dívidas clandestinas."
		}
		cross := suspect.CrossSuspicion
		if cross == "" {
			cross = "Você deveria olhar com mais atenção para os outros presentes na casa!"
		}
		return fmt.Sprintf("O suspeito hesita sob pressão, cerra os dentes e admite: 'Tudo bem, detetive! Eu confesso que %s Mas não matei ninguém! %s'", petty, cross)
	}

	// Low Stress (<30%): Tier 1 Alibi and confident composure
	if suspect.Tier1Alibi != "" {
		if containsAny(q, "arma", "adaga", "punhal", "faca", "veneno", "sangue", "morte", "matou", "assassin") {
			return GenerateFallbackResponse(suspect, question)
		}
		if containsAny(q, "motivo", "dinheiro", "testamento", "dívida", "herança", "fortuna", "segredo", "por que") {
			return GenerateFallbackResponse(suspect, question)
		}
		if strings.TrimSpace(suspect.CrossSuspicion) != "" {
			return fmt.Sprintf("%s Além disso, %s", suspect.Tier1Alibi, suspect.CrossSuspicion)
		}
		return suspect.Tier1Alibi
	}

	return GenerateFallbackResponse(suspect, question)
}

// GenerateFallbackResponse generates an in-character Noir response without external dependencies.
func GenerateFallbackResponse(suspect *models.Suspect, question string) string {
	q := strings.ToLower(question)

	switch {
	// Questions about alibi, whereabouts, night of the crime
	case containsAny(q, "alibi", "álibi", "onde", "estava", "noite", "hora", "horário", "quando"):
		if suspect.Tier1Alibi != "" {
			return suspect.Tier1Alibi
		}
		if suspect.Alibi != "" {
			return suspect.Alibi
		}
		return fmt.Sprintf("Como %s, permaneci em meus aposentos cumprindo minhas obrigações. Não tenho qualquer envolvimento com este crime.", suspect.Role)

	// Questions about the murder weapon, dagger, poison, blood
	case containsAny(q, "arma", "adaga", "punhal", "faca", "veneno", "sangue", "morte", "matou", "assassin", "castiçal", "navalha"):
		if suspect.IsCulprit && suspect.Tier2PettyCrime != "" {
			return fmt.Sprintf("Arma do crime? Detetive, você está perdendo seu tempo comigo. %s", suspect.Tier2PettyCrime)
		}
		return fmt.Sprintf("Como %s, não ando armado e repudio essa violência brutal. Meus instrumentos de trabalho são outros.", suspect.Role)

	// Questions about motive, money, will, inheritance, debts
	case containsAny(q, "motivo", "dinheiro", "testamento", "dívida", "herança", "fortuna", "segredo", "por que"):
		if suspect.Tier2PettyCrime != "" {
			return fmt.Sprintf("Todo mundo tem assuntos que prefere manter em segredo... %s Mas disso a um homicídio há um abismo!", suspect.Tier2PettyCrime)
		}
		if suspect.Secret != "" {
			return "Meus assuntos particulares não dizem respeito a este inquérito, detetive. Minha conduta perante a vítima sempre foi profissional."
		}
		return "Não tenho nenhum motivo para desejar a morte da vítima. Minha relação era estritamente formal."

	// Default in-character response
	default:
		if strings.TrimSpace(suspect.CrossSuspicion) != "" {
			return fmt.Sprintf("Detetive, meça suas insinuações. Sou %s. %s", suspect.Role, suspect.CrossSuspicion)
		}
		if suspect.Personality != "" {
			return fmt.Sprintf("Sou %s. %s Já respondi a tudo o que a polícia tinha direito de saber.", suspect.Role, suspect.Personality)
		}
		return fmt.Sprintf("Sou %s. Estou à inteira disposição da investigação, desde que não façam ilações infundadas.", suspect.Role)
	}
}

func containsAny(text string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}
