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
	"time"

	"detetive-ia/internal/models"
)

// AIClient defines the contract for LLM generation.
type AIClient interface {
	GenerateResponse(ctx context.Context, prompt string) (string, error)
}

// GeminiClient implements AIClient using Google's Gemini REST API.
type GeminiClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewGeminiClient creates a lightweight Gemini API client.
func NewGeminiClient(apiKey string) *GeminiClient {
	if apiKey == "" {
		return nil
	}
	return &GeminiClient{
		apiKey: apiKey,
		model:  "gemini-1.5-flash",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (g *GeminiClient) GenerateResponse(ctx context.Context, prompt string) (string, error) {
	if g.apiKey == "" {
		return "", errors.New("gemini api key is not configured")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", g.model, g.apiKey)

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
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

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("gemini api error (status %d): %s", resp.StatusCode, string(bodyBytes))
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

	return "", errors.New("empty response from gemini api")
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
		if strings.Contains(suspect.Name, "Julian") {
			return "Detetive, como já afirmei à polícia, passei a noite inteira na sala de música. Dedilhava o piano de cauda com uma taça de conhaque ao meu lado. Não saí de lá até ouvir a comoção."
		} else if strings.Contains(suspect.Name, "Elena") {
			return "Estava no meu laboratório particular, catalogando dosagens e amostras botânicas. A ciência exige precisão e isolamento. Ninguém entrou ou saiu enquanto eu preparava os frascos."
		} else {
			return "Eu me encontrava na adega principal, senhor. Arthur exigia o inventário anual das safras raras de Bordeaux. Posso apresentar as notas fiscais e registros de temperatura se assim desejar."
		}

	// Questions about the murder weapon, dagger, poison, blood
	case containsAny(q, "arma", "adaga", "punhal", "faca", "veneno", "sangue", "morte", "matou", "assassin"):
		if strings.Contains(suspect.Name, "Julian") {
			return "Uma adaga cerimonial veneziana? Ora, Arthur tinha centenas de relíquias grotescas espalhadas pela casa. Por que eu tocaria em algo tão bárbaro quando herdei toda a linhagem Pendelton?"
		} else if strings.Contains(suspect.Name, "Elena") {
			return "Como médica legista e pesquisadora, observei o corte limpo e profundo na carótida. Foi obra de alguém com pulso firme e lâmina afiada. Meus bisturis permanecem trancados sob chave."
		} else {
			return "A adaga pertencia à coleção privada de antiguidades da família há três gerações. Ficava exposta na vitrine da biblioteca. Notei sua ausência apenas quando o corpo foi descoberto."
		}

	// Questions about motive, money, will, inheritance, debts
	case containsAny(q, "motivo", "dinheiro", "testamento", "dívida", "herança", "fortuna", "segredo", "por que"):
		if strings.Contains(suspect.Name, "Julian") {
			return "Isso é um ultraje! Minhas finanças são assuntos estritamente pessoais da alta sociedade. Arthur era meu tio e me prometeu tudo... ele não teria coragem de me deserdar por causa de especulações tolas de cassino!"
		} else if strings.Contains(suspect.Name, "Elena") {
			return "Arthur financiava minhas pesquisas médicas. Tínhamos divergências sobre a aplicação dos orçamentos, mas o trabalho era vital. Jamais destruiria a fonte dos meus estudos."
		} else {
			return "Sir Arthur era um homem exigente e por vezes tempestuoso. Eu conhecia todos os seus segredos e correspondências confidenciais, mas minha lealdade à casa Pendelton foi inabalável por quarenta anos."
		}

	// Questions about other suspects
	case strings.Contains(q, "elena"):
		if strings.Contains(suspect.Name, "Julian") {
			return "A Dra. Rostov sempre teve um olhar calculista demais para alguém que cuida de vidas humanas. Ela sussurrava com Arthur a portas fechadas com frequência incomum."
		}
		return "A doutora mantém uma postura rigorosa, mas notei seu nervosismo recente nos corredores."

	case strings.Contains(q, "barnaby"):
		if strings.Contains(suspect.Name, "Julian") {
			return "O velho Barnaby conhece cada fresta e passagem secreta desta mansão. Uma sombra que ouve tudo e guarda rancores antigos."
		}
		return "Barnaby é discreto como convém a um bom mordomo, mas sabe mais do que aparenta revelar."

	case strings.Contains(q, "julian"):
		if strings.Contains(suspect.Name, "Elena") {
			return "Lorde Julian é um dândi irresponsável, afundado em dívidas de jogo. Ouvi uma discussão acalorada entre ele e Arthur na véspera do crime."
		}
		return "O jovem mestre Julian sempre foi perdulário. Os credores têm rondado a propriedade nas últimas semanas."

	// Default in-character response
	default:
		if strings.Contains(suspect.Name, "Julian") {
			return "Meça suas palavras, detetive. Acusações infundadas contra a aristocracia custam caro. O que mais deseja saber para encerrar este incômodo?"
		} else if strings.Contains(suspect.Name, "Elena") {
			return "Os fatos e a evidência material falarão mais alto do que conjecturas dramáticas. Tem alguma pergunta concreta para mim?"
		} else {
			return "Estou à inteira disposição da investigação, senhor. Apenas desejo que a justiça por Sir Arthur seja feita com a devida sobriedade."
		}
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
