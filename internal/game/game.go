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
type CaseDetails = models.CaseDetails


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

// GetAIClient returns the current LLM client.
func (s *Service) GetAIClient() AIClient {
	return s.aiClient
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
		name           string
		role           string
		personality    string
		alibi          string
		secret         string
		isCulprit      bool
		tier1Alibi     string
		tier2Petty     string
		tier3Truth     string
		crossSuspicion string
	}{
		{
			name:           "Lorde Julian Vane",
			role:           "Sobrinho & Herdeiro",
			personality:    "Elegante, arrogante e nervoso sob pressão financeira.",
			alibi:          "Afirma que dedilhava o piano de cauda na sala de música tomando conhaque.",
			secret:         "Falido por dívidas clandestinas de jogo, seria deserdado na manhã seguinte.",
			isCulprit:      true,
			tier1Alibi:     "Dedilhava o piano de cauda na sala de música tomando conhaque.",
			tier2Petty:     "Devo 50.000 francos a agiotas do submundo e penhorei joias da família.",
			tier3Truth:     "Arthur descobriu tudo e ia me deserdar hoje de manhã! Eu perdi a cabeça quando ele puxou a carta de deserdação!",
			crossSuspicion: "A Dra. Elena estava preparando venenos proibidos no laboratório e discutiu aos gritos com Arthur.",
		},
		{
			name:           "Dra. Elena Rostov",
			role:           "Médica Pessoal & Pesquisadora",
			personality:    "Fria, metódica e defensiva sobre dosagens e frascos de remédio.",
			alibi:          "Afirma que estava no laboratório organizando medicamentos e amostras.",
			secret:         "Desviava fundos da caridade de Arthur para pesquisas proibidas.",
			isCulprit:      false,
			tier1Alibi:     "Estava no laboratório organizando medicamentos e amostras botânicas.",
			tier2Petty:     "Desviava fundos da caridade de Arthur para pesquisas toxicológicas não autorizadas.",
			tier3Truth:     "Eu estava no laboratório até meia-noite, mas vi Julian sair da biblioteca com as mãos trêmulas!",
			crossSuspicion: "Lorde Julian é um dândi desesperado, ele implorava dinheiro a Arthur horas antes do assassinato.",
		},
		{
			name:           "Barnaby Finch",
			role:           "Mordomo & Guarda-livros",
			personality:    "Formal, taciturno e detalhista.",
			alibi:          "Afirma que estava na adega conferindo safras raras de vinho.",
			secret:         "Guardava correspondências comprometedoras de Arthur para evitar demissão.",
			isCulprit:      false,
			tier1Alibi:     "Estava na adega conferindo safras raras de vinho e cristais.",
			tier2Petty:     "Guardava correspondências comprometedoras de Arthur para me proteger de demissão arbitrária.",
			tier3Truth:     "Servi a família por 40 anos e nunca ergueria a mão contra meu patrão.",
			crossSuspicion: "Julian estava rondando a vitrine da adaga cerimonial durante o jantar.",
		},
	}

	var culpritID int64
	for _, susp := range suspects {
		sRes, err := s.db.Exec(`
			INSERT INTO suspects (case_id, name, role, personality, alibi, secret, is_culprit, tier1_alibi, tier2_petty_crime, tier3_truth, cross_suspicion)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, caseID, susp.name, susp.role, susp.personality, susp.alibi, susp.secret, susp.isCulprit,
			susp.tier1Alibi, susp.tier2Petty, susp.tier3Truth, susp.crossSuspicion)
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
		SELECT id, case_id, name, role, personality, alibi, secret, is_culprit,
		       COALESCE(tier1_alibi, ''), COALESCE(tier2_petty_crime, ''), COALESCE(tier3_truth, ''), COALESCE(cross_suspicion, ''),
		       COALESCE(gender, ''), COALESCE(avatar, '')
		FROM suspects WHERE case_id = ? ORDER BY id ASC
	`, caseID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch suspects: %w", err)
	}
	defer suspectRows.Close()

	var suspects []models.Suspect
	for suspectRows.Next() {
		var susp models.Suspect
		if err := suspectRows.Scan(&susp.ID, &susp.CaseID, &susp.Name, &susp.Role, &susp.Personality, &susp.Alibi, &susp.Secret, &susp.IsCulprit,
			&susp.Tier1Alibi, &susp.Tier2PettyCrime, &susp.Tier3Truth, &susp.CrossSuspicion, &susp.Gender, &susp.Avatar); err != nil {
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
	return s.InterrogateWithStress(sessionID, suspectID, question, 15)
}

// InterrogateWithStress asks a question to a suspect factoring in stress levels and 3-tier narrative secrets.
func (s *Service) InterrogateWithStress(sessionID string, suspectID int64, question string, stress int) (string, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return "", errors.New("question cannot be empty")
	}

	var susp models.Suspect
	err := s.db.QueryRow(`
		SELECT id, case_id, name, role, personality, alibi, secret, is_culprit,
		       COALESCE(tier1_alibi, ''), COALESCE(tier2_petty_crime, ''), COALESCE(tier3_truth, ''), COALESCE(cross_suspicion, ''),
		       COALESCE(gender, ''), COALESCE(avatar, '')
		FROM suspects WHERE id = ?
	`, suspectID).Scan(&susp.ID, &susp.CaseID, &susp.Name, &susp.Role, &susp.Personality, &susp.Alibi, &susp.Secret, &susp.IsCulprit,
		&susp.Tier1Alibi, &susp.Tier2PettyCrime, &susp.Tier3Truth, &susp.CrossSuspicion, &susp.Gender, &susp.Avatar)
	if err != nil {
		return "", fmt.Errorf("failed to find suspect: %w", err)
	}

	var answer string
	if s.aiClient != nil {
		prompt := fmt.Sprintf(
			"Você é %s (%s). Personalidade: %s. Álibi (Tier 1): %s. Delito Secundário (Tier 2): %s. Verdade Íntima (Tier 3): %s. Suspeita sobre outro personagem: %s. Nível de estresse: %d/100. Responda em Português como um personagem Noir à pergunta do Detetive: '%s'. Mantenha a resposta concisa, expressiva e misteriosa. Se o estresse for <30%%, mantenha seu álibi. Se estiver entre 30%% e 70%%, admita com relutância seu delito secundário e lance suspeita sobre outro. Se estiver acima de 70%%, quebre e revele a verdade íntima.",
			susp.Name, susp.Role, susp.Personality, susp.Tier1Alibi, susp.Tier2PettyCrime, susp.Tier3Truth, susp.CrossSuspicion, stress, question,
		)
		resp, err := s.aiClient.GenerateResponse(context.Background(), prompt)
		if err == nil && len(strings.TrimSpace(resp)) > 0 {
			answer = resp
		}
	}

	if answer == "" {
		answer = GenerateFallbackResponseWithStress(&susp, question, stress)
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

	wLower := strings.TrimSpace(strings.ToLower(accusedWeapon))
	solWLower := strings.TrimSpace(strings.ToLower(solWeapon))
	weaponMatch := matchWeapon(wLower, solWLower)

	mLower := strings.TrimSpace(strings.ToLower(accusedMotive))
	solMLower := strings.TrimSpace(strings.ToLower(solMotive))
	motiveMatch := false
	if mLower != "" && solMLower != "" {
		motiveMatch = strings.Contains(mLower, solMLower) || strings.Contains(solMLower, mLower) || containsAny(mLower, "herança", "heranca", "dívida", "divida", "dinheiro", "jogo", "cassino", "chantagem", "vingança", "vinganca", "desvio")
	}

	var verdict models.Verdict

	if (accusedSuspectID == solSuspectID || isCulprit) && weaponMatch && motiveMatch {
		verdict = models.Verdict{
			Correct:       true,
			Score:         100,
			Feedback:      fmt.Sprintf("Dedução impecável, Detetive! Você desmascarou %s, identificou a %s como arma do crime e comprovou a motivação de %s.", suspName, solWeapon, solMotive),
			ActualCulprit: suspName,
			ActualWeapon:  solWeapon,
			ActualMotive:  solMotive,
		}
	} else if accusedSuspectID == solSuspectID || isCulprit {
		verdict = models.Verdict{
			Correct:       false,
			Score:         50,
			Feedback:      fmt.Sprintf("Você apontou o culpado correto (%s), mas falhou na comprovação pericial da arma ou do motivo real perante o tribunal.", suspName),
			ActualCulprit: suspName,
			ActualWeapon:  solWeapon,
			ActualMotive:  solMotive,
		}
	} else {
		verdict = models.Verdict{
			Correct:       false,
			Score:         10,
			Feedback:      fmt.Sprintf("Acusação incorreta! %s possuía um álibi corroborado e o verdadeiro assassino escapou ileso.", suspName),
			ActualCulprit: "Desconhecido",
			ActualWeapon:  "Não comprovada",
			ActualMotive:  "Incógnita",
		}
	}

	if userID > 0 && verdict.Correct {
		var invID int64
		_ = s.db.QueryRow(`SELECT id FROM investigations WHERE user_id = ? AND case_id = ? AND status = 'in_progress' ORDER BY id DESC LIMIT 1`, userID, caseID).Scan(&invID)
		if invID > 0 {
			_, _ = s.db.Exec(`
				UPDATE investigations
				SET status = 'solved', accused_suspect_id = ?, accused_weapon = ?, accused_motive = ?, completed_at = CURRENT_TIMESTAMP
				WHERE id = ?
			`, accusedSuspectID, accusedWeapon, accusedMotive, invID)
		} else {
			_, _ = s.db.Exec(`
				INSERT INTO investigations (user_id, case_id, status, accused_suspect_id, accused_weapon, accused_motive, completed_at, time_remaining_minutes, warrants_remaining)
				VALUES (?, ?, 'solved', ?, ?, ?, CURRENT_TIMESTAMP, 420, 2)
			`, userID, caseID, accusedSuspectID, accusedWeapon, accusedMotive)
		}
	}

	return &verdict, nil
}

// CreateInvestigationForCase creates a fresh in-progress investigation with 420 minutes and 2 warrants for the given user and case.
func (s *Service) CreateInvestigationForCase(userID int64, caseID int64) (*models.Investigation, error) {
	var userVal interface{} = userID
	if userID <= 0 {
		userVal = nil
	}

	res, err := s.db.Exec(`
		INSERT INTO investigations (user_id, case_id, status, time_remaining_minutes, warrants_remaining)
		VALUES (?, ?, 'in_progress', 420, 2)
	`, userVal, caseID)
	if err != nil {
		return nil, fmt.Errorf("failed to create investigation: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &models.Investigation{
		ID:                   id,
		UserID:               userID,
		CaseID:               caseID,
		Status:               "in_progress",
		TimeRemainingMinutes: 420,
		WarrantsRemaining:    2,
	}, nil
}

// GetOrCreateInvestigation finds an active in-progress investigation for the given user and case, or creates a new one.
func (s *Service) GetOrCreateInvestigation(userID int64, caseID int64) (*models.Investigation, error) {
	var inv models.Investigation
	var query string
	var args []interface{}

	if userID > 0 {
		query = `SELECT id, user_id, case_id, status, time_remaining_minutes, warrants_remaining FROM investigations WHERE user_id = ? AND case_id = ? AND status = 'in_progress' ORDER BY id DESC LIMIT 1`
		args = []interface{}{userID, caseID}
	} else {
		query = `SELECT id, COALESCE(user_id, 0), case_id, status, time_remaining_minutes, warrants_remaining FROM investigations WHERE (user_id IS NULL OR user_id = 0) AND case_id = ? AND status = 'in_progress' ORDER BY id DESC LIMIT 1`
		args = []interface{}{caseID}
	}

	err := s.db.QueryRow(query, args...).Scan(&inv.ID, &inv.UserID, &inv.CaseID, &inv.Status, &inv.TimeRemainingMinutes, &inv.WarrantsRemaining)
	if err == nil {
		return &inv, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to query investigation: %w", err)
	}

	return s.CreateInvestigationForCase(userID, caseID)
}

// GetLatestInvestigation retrieves the most recent investigation for a user and case, regardless of status.
func (s *Service) GetLatestInvestigation(userID int64, caseID int64) (*models.Investigation, error) {
	var inv models.Investigation
	var query string
	var args []interface{}

	if userID > 0 {
		query = `SELECT id, user_id, case_id, status, time_remaining_minutes, warrants_remaining FROM investigations WHERE user_id = ? AND case_id = ? ORDER BY id DESC LIMIT 1`
		args = []interface{}{userID, caseID}
	} else {
		query = `SELECT id, COALESCE(user_id, 0), case_id, status, time_remaining_minutes, warrants_remaining FROM investigations WHERE (user_id IS NULL OR user_id = 0) AND case_id = ? ORDER BY id DESC LIMIT 1`
		args = []interface{}{caseID}
	}

	err := s.db.QueryRow(query, args...).Scan(&inv.ID, &inv.UserID, &inv.CaseID, &inv.Status, &inv.TimeRemainingMinutes, &inv.WarrantsRemaining)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}


// StartSoloGame starts a new solo procedural game with clean investigation metrics (420 min / 2 warrants).
func (s *Service) StartSoloGame(userID int64, themeID string, suspectCount int) (*models.CaseDetails, *models.Investigation, error) {
	if themeID == "" {
		themeID = "mansion"
	}
	if suspectCount <= 0 {
		suspectCount = 4
	}

	weaver := NewWeaver(s.db)
	res, err := weaver.WeaveCaseWithTheme(nil, themeID, suspectCount)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to weave solo case: %w", err)
	}

	details, err := s.GetCaseDetails(res.CaseID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get case details: %w", err)
	}

	inv, err := s.CreateInvestigationForCase(userID, res.CaseID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create investigation: %w", err)
	}

	return details, inv, nil
}


// GetInvestigation retrieves an investigation by ID.
func (s *Service) GetInvestigation(investigationID int64) (*models.Investigation, error) {
	var inv models.Investigation
	err := s.db.QueryRow(`
		SELECT id, COALESCE(user_id, 0), case_id, status, time_remaining_minutes, warrants_remaining
		FROM investigations WHERE id = ?
	`, investigationID).Scan(&inv.ID, &inv.UserID, &inv.CaseID, &inv.Status, &inv.TimeRemainingMinutes, &inv.WarrantsRemaining)
	if err != nil {
		return nil, fmt.Errorf("investigation not found: %w", err)
	}
	return &inv, nil
}

// DeductTime decreases time remaining on an investigation and marks it failed if time reaches zero.
func (s *Service) DeductTime(investigationID int64, minutes int) (int, error) {
	inv, err := s.GetInvestigation(investigationID)
	if err != nil {
		return 0, err
	}

	newTime := inv.TimeRemainingMinutes - minutes
	status := inv.Status
	if newTime <= 0 {
		newTime = 0
		status = "failed"
	}

	_, err = s.db.Exec(`
		UPDATE investigations
		SET time_remaining_minutes = ?, status = ?
		WHERE id = ?
	`, newTime, status, investigationID)
	if err != nil {
		return 0, fmt.Errorf("failed to update time remaining: %w", err)
	}

	return newTime, nil
}

// RecordWarrantStrike decrements warrants remaining by 1. If warrants reach zero, marks as failed and returns isGameOver=true.
func (s *Service) RecordWarrantStrike(investigationID int64) (int, bool, error) {
	inv, err := s.GetInvestigation(investigationID)
	if err != nil {
		return 0, false, err
	}

	warrantsLeft := inv.WarrantsRemaining - 1
	isGameOver := false
	status := inv.Status

	if warrantsLeft <= 0 {
		warrantsLeft = 0
		isGameOver = true
		status = "failed"
	}

	_, err = s.db.Exec(`
		UPDATE investigations
		SET warrants_remaining = ?, status = ?
		WHERE id = ?
	`, warrantsLeft, status, investigationID)
	if err != nil {
		return 0, false, fmt.Errorf("failed to update warrants remaining: %w", err)
	}

	return warrantsLeft, isGameOver, nil
}

func matchWeapon(accused, solution string) bool {
	acc := strings.ToLower(strings.TrimSpace(accused))
	sol := strings.ToLower(strings.TrimSpace(solution))
	if acc == "" || sol == "" {
		return false
	}
	if strings.Contains(acc, sol) || strings.Contains(sol, acc) {
		return true
	}
	weapons := []struct {
		solKeywords []string
		accKeywords []string
	}{
		{[]string{"adaga"}, []string{"adaga"}},
		{[]string{"cianeto", "cálice", "calice"}, []string{"cianeto", "cálice", "calice", "veneno"}},
		{[]string{"revólver", "revolver", ".38"}, []string{"revólver", "revolver", ".38", "arma"}},
		{[]string{"castiçal", "castical"}, []string{"castiçal", "castical", "bronze"}},
		{[]string{"navalha"}, []string{"navalha", "lâmina", "lamina"}},
	}
	for _, w := range weapons {
		solMatches := false
		for _, s := range w.solKeywords {
			if strings.Contains(sol, s) {
				solMatches = true
				break
			}
		}
		if solMatches {
			for _, a := range w.accKeywords {
				if strings.Contains(acc, a) {
					return true
				}
			}
		}
	}
	return false
}
