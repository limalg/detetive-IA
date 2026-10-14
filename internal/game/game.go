package game

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"detetive-ia/internal/models"
)

// CaseDetails aggregates the crime case data, including suspects and discovered clues.
type CaseDetails struct {
	Case     models.Case      `json:"case"`
	Suspects []models.Suspect `json:"suspects"`
	Clues    []models.Clue    `json:"clues"`
}

// Service handles mystery progression, suspect interrogation, and verdict evaluation.
type Service struct {
	db       *sql.DB
	aiClient AIClient
}

// New creates a new game service instance.
func New(db *sql.DB, aiClient AIClient) *Service {
	return &Service{
		db:       db,
		aiClient: aiClient,
	}
}

// SetAIClient allows dynamic injection of LLM client or test mocks.
func (s *Service) SetAIClient(aiClient AIClient) {
	s.aiClient = aiClient
}

// SeedInitialCase populates the initial murder mystery "O Enigma do Vale das Sombras" if not already present.
func (s *Service) SeedInitialCase() (int64, error) {
	var existingID int64
	checkQuery := `SELECT id FROM cases WHERE title = ? LIMIT 1`
	err := s.db.QueryRow(checkQuery, "O Enigma do Vale das Sombras").Scan(&existingID)
	if err == nil {
		return existingID, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("failed to check existing case: %w", err)
	}

	caseInsert := `
		INSERT INTO cases (title, description, victim, solution_weapon, solution_motive)
		VALUES (?, ?, ?, ?, ?)
	`
	desc := "Arthur Pendelton, renomado colecionador de antiguidades e filantropo, foi encontrado morto em sua biblioteca privada na mansão do Vale das Sombras. A porta estava trancada por dentro com chave mestre. Três suspeitos estavam presentes na noite do crime."
	victim := "Arthur Pendelton"
	weapon := "Adaga Cerimonial Veneziana"
	motive := "Herança e dívidas de jogo secretas"

	res, err := s.db.Exec(caseInsert, "O Enigma do Vale das Sombras", desc, victim, weapon, motive)
	if err != nil {
		return 0, fmt.Errorf("failed to insert initial case: %w", err)
	}

	caseID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	suspects := []struct {
		name        string
		role        string
		personality string
		alibi       string
		secret      string
		isCulprit   bool
	}{
		{
			name:        "Lorde Julian Vane",
			role:        "Sobrinho & Herdeiro",
			personality: "Elegante, arrogante e nervoso sob pressão financeira.",
			alibi:       "Afirma que dedilhava o piano de cauda na sala de música tomando conhaque.",
			secret:      "Falido por dívidas clandestinas de jogo, seria deserdado na manhã seguinte.",
			isCulprit:   true,
		},
		{
			name:        "Dra. Elena Rostov",
			role:        "Médica Pessoal & Pesquisadora",
			personality: "Fria, metódica e defensiva sobre dosagens e frascos de remédio.",
			alibi:       "Afirma que estava no laboratório organizando medicamentos e amostras.",
			secret:      "Desviava fundos da caridade de Arthur para pesquisas proibidas.",
			isCulprit:   false,
		},
		{
			name:        "Barnaby Finch",
			role:        "Mordomo & Guarda-livros",
			personality: "Formal, taciturno e detalhista.",
			alibi:       "Afirma que estava na adega conferindo safras raras de vinho.",
			secret:      "Guardava correspondências comprometedoras de Arthur para evitar demissão.",
			isCulprit:   false,
		},
	}

	var culpritID int64
	for _, susp := range suspects {
		sRes, err := s.db.Exec(`
			INSERT INTO suspects (case_id, name, role, personality, alibi, secret, is_culprit)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, caseID, susp.name, susp.role, susp.personality, susp.alibi, susp.secret, susp.isCulprit)
		if err != nil {
			return 0, fmt.Errorf("failed to insert suspect %s: %w", susp.name, err)
		}
		sID, _ := sRes.LastInsertId()
		if susp.isCulprit {
			culpritID = sID
		}
	}

	// Associate solution_suspect_id with the case
	if culpritID > 0 {
		_, _ = s.db.Exec(`UPDATE cases SET solution_suspect_id = ? WHERE id = ?`, culpritID, caseID)
	}

	clues := []struct {
		location string
		name     string
		desc     string
	}{
		{
			location: "Biblioteca",
			name:     "Adaga Cerimonial Veneziana",
			desc:     "Lâmina ornamentada com pó dourado e vestígios de sangue no cabo de madrepérola.",
		},
		{
			location: "Sala de Música",
			name:     "Copo de Conhaque & Luva",
			desc:     "Copo intocado e uma luva de veludo esmeralda esquecida sobre as teclas do piano.",
		},
		{
			location: "Escrivaninha Privada",
			name:     "Testamento Rasgado",
			desc:     "Rascunho de testamento alterando todos os herdeiros com a data da própria manhã seguinte.",
		},
		{
			location: "Jardim de Inverno",
			name:     "Pegadas e Cinzas",
			desc:     "Pegadas enlameadas de sapatos caros de couro italiano tamanho 42, com cinzas de charuto cubano.",
		},
	}

	for _, clue := range clues {
		_, err := s.db.Exec(`
			INSERT INTO clues (case_id, location, name, description)
			VALUES (?, ?, ?, ?)
		`, caseID, clue.location, clue.name, clue.desc)
		if err != nil {
			return 0, fmt.Errorf("failed to insert clue %s: %w", clue.name, err)
		}
	}

	return caseID, nil
}

// GetCaseDetails fetches all case data, suspects, and clues.
func (s *Service) GetCaseDetails(caseID int64) (*CaseDetails, error) {
	var c models.Case
	err := s.db.QueryRow(`
		SELECT id, title, description, victim, solution_suspect_id, solution_weapon, solution_motive
		FROM cases WHERE id = ?
	`, caseID).Scan(&c.ID, &c.Title, &c.Description, &c.Victim, &c.SolutionSuspectID, &c.SolutionWeapon, &c.SolutionMotive)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch case: %w", err)
	}

	suspectRows, err := s.db.Query(`
		SELECT id, case_id, name, role, personality, alibi, secret, is_culprit
		FROM suspects WHERE case_id = ? ORDER BY id ASC
	`, caseID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch suspects: %w", err)
	}
	defer suspectRows.Close()

	var suspects []models.Suspect
	for suspectRows.Next() {
		var susp models.Suspect
		if err := suspectRows.Scan(&susp.ID, &susp.CaseID, &susp.Name, &susp.Role, &susp.Personality, &susp.Alibi, &susp.Secret, &susp.IsCulprit); err != nil {
			return nil, err
		}
		suspects = append(suspects, susp)
	}

	clueRows, err := s.db.Query(`
		SELECT id, case_id, location, name, description
		FROM clues WHERE case_id = ? ORDER BY id ASC
	`, caseID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch clues: %w", err)
	}
	defer clueRows.Close()

	var clues []models.Clue
	for clueRows.Next() {
		var clue models.Clue
		if err := clueRows.Scan(&clue.ID, &clue.CaseID, &clue.Location, &clue.Name, &clue.Description); err != nil {
			return nil, err
		}
		clues = append(clues, clue)
	}

	return &CaseDetails{
		Case:     c,
		Suspects: suspects,
		Clues:    clues,
	}, nil
}

// Interrogate asks a question to a suspect and returns their dynamic or fallback response.
func (s *Service) Interrogate(sessionID string, suspectID int64, question string) (string, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return "", errors.New("question cannot be empty")
	}

	var susp models.Suspect
	err := s.db.QueryRow(`
		SELECT id, case_id, name, role, personality, alibi, secret, is_culprit
		FROM suspects WHERE id = ?
	`, suspectID).Scan(&susp.ID, &susp.CaseID, &susp.Name, &susp.Role, &susp.Personality, &susp.Alibi, &susp.Secret, &susp.IsCulprit)
	if err != nil {
		return "", fmt.Errorf("failed to find suspect: %w", err)
	}

	var answer string
	if s.aiClient != nil {
		prompt := fmt.Sprintf(
			"Você é %s (%s). Sua personalidade: %s. Seu álibi: %s. Seu segredo íntimo: %s. Você é o culpado? %v. Responda em Português como um personagem Noir à pergunta do Detetive: '%s'. Mantenha a resposta concisa, expressiva e misteriosa.",
			susp.Name, susp.Role, susp.Personality, susp.Alibi, susp.Secret, susp.IsCulprit, question,
		)
		resp, err := s.aiClient.GenerateResponse(context.Background(), prompt)
		if err == nil && len(strings.TrimSpace(resp)) > 0 {
			answer = resp
		}
	}

	if answer == "" {
		answer = GenerateFallbackResponse(&susp, question)
	}

	// Persist dialogue exchange
	_, _ = s.db.Exec(`
		INSERT INTO interrogation_messages (session_id, suspect_id, sender, message)
		VALUES (?, ?, 'detective', ?)
	`, sessionID, suspectID, question)

	_, _ = s.db.Exec(`
		INSERT INTO interrogation_messages (session_id, suspect_id, sender, message)
		VALUES (?, ?, 'suspect', ?)
	`, sessionID, suspectID, answer)

	return answer, nil
}

// SubmitAccusation evaluates the detective's deduction and records the investigation outcome.
func (s *Service) SubmitAccusation(userID int64, caseID int64, accusedSuspectID int64, accusedWeapon string, accusedMotive string) (*models.Verdict, error) {
	var solSuspectID int64
	var solWeapon, solMotive string
	err := s.db.QueryRow(`
		SELECT solution_suspect_id, solution_weapon, solution_motive
		FROM cases WHERE id = ?
	`, caseID).Scan(&solSuspectID, &solWeapon, &solMotive)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch case solution: %w", err)
	}

	var suspName string
	var isCulprit bool
	err = s.db.QueryRow(`SELECT name, is_culprit FROM suspects WHERE id = ?`, accusedSuspectID).Scan(&suspName, &isCulprit)
	if err != nil {
		return nil, fmt.Errorf("suspect not found: %w", err)
	}

	wLower := strings.ToLower(accusedWeapon)
	solWLower := strings.ToLower(solWeapon)
	weaponMatch := strings.Contains(wLower, "adaga") || strings.Contains(wLower, solWLower) || strings.Contains(solWLower, wLower)

	mLower := strings.ToLower(accusedMotive)
	motiveMatch := containsAny(mLower, "herança", "heranca", "dívida", "divida", "dinheiro", "jogo", "cassino")

	var verdict models.Verdict
	var status string

	if (accusedSuspectID == solSuspectID || isCulprit) && weaponMatch && motiveMatch {
		verdict = models.Verdict{
			Correct:       true,
			Score:         100,
			Feedback:      fmt.Sprintf("Dedução impecável, Detetive! Você desmascarou %s, identificou a %s como arma do crime e comprovou a motivação de %s.", suspName, solWeapon, solMotive),
			ActualCulprit: suspName,
			ActualWeapon:  solWeapon,
			ActualMotive:  solMotive,
		}
		status = "solved"
	} else if accusedSuspectID == solSuspectID || isCulprit {
		verdict = models.Verdict{
			Correct:       false,
			Score:         50,
			Feedback:      fmt.Sprintf("Você apontou o culpado correto (%s), mas falhou na comprovação pericial da arma ou do motivo real perante o tribunal.", suspName),
			ActualCulprit: suspName,
			ActualWeapon:  solWeapon,
			ActualMotive:  solMotive,
		}
		status = "failed"
	} else {
		verdict = models.Verdict{
			Correct:       false,
			Score:         10,
			Feedback:      fmt.Sprintf("Acusação incorreta! %s possuía um álibi corroborado e o verdadeiro assassino escapou ileso.", suspName),
			ActualCulprit: "Desconhecido",
			ActualWeapon:  "Não comprovada",
			ActualMotive:  "Incógnita",
		}
		status = "failed"
	}

	if userID > 0 {
		_, _ = s.db.Exec(`
			INSERT INTO investigations (user_id, case_id, status, accused_suspect_id, accused_weapon, accused_motive, completed_at)
			VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, userID, caseID, status, accusedSuspectID, accusedWeapon, accusedMotive)
	}

	return &verdict, nil
}
