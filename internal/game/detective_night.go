package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"detetive-ia/internal/models"
	"detetive-ia/internal/telemetry"
)

// DetectiveNightCaseTemplate defines the data for a Detective Night board-game-to-digital case.
type DetectiveNightCaseTemplate struct {
	Title          string
	Description    string
	Victim         string
	SolutionWeapon string
	SolutionMotive string
	Envelopes      []models.CaseEnvelope
	DefaultSuspects []models.Suspect
	DefaultClues   []models.Clue
}

// GetDetectiveNightCatalog returns the pre-authored digital adaptations of Detective Night.
func GetDetectiveNightCatalog() []DetectiveNightCaseTemplate {
	return []DetectiveNightCaseTemplate{
		{
			Title:          "Mistério na Fazenda Coração da Serra",
			Description:    "O fazendeiro Antônio Ferraz foi assassinado por envenenamento em seu escritório horas antes de assinar seu novo testamento. Explore as pastas confidenciais, confronte os herdeiros e desmascare a farsa.",
			Victim:         "Antônio Ferraz",
			SolutionWeapon: "Veneno (Cianeto no Vinho do Porto)",
			SolutionMotive: "Herança e deserdamento iminente",
			Envelopes: []models.CaseEnvelope{
				{
					EnvelopeNumber: 1,
					Title:          "Pasta 1 – O Caso",
					Description:    "Dossiê inicial, transcrição da chamada de emergência 190, laudo do legista e fotos periciais do escritório.",
					ContentJSON: `{
						"tipo": "dossie_inicial",
						"chamada_emergencia": "22h45 - 'Socorro, por favor! O patrão Antônio desmaiou após o brinde, está roxo, não está respirando!'",
						"laudo_legista": "Causa da morte: Asfixia tóxica por ingestão de cianeto diluído em líquido alcoólico doce. Rigor mortis iniciado às 22h15.",
						"cena_do_crime": "Escritório principal trancado por dentro pela chave mestra. Copo de cristal com restos de vinho, lareira acesa com cinzas de papel timbrado.",
						"noticia_jornal": "Tribuna Regional: Morte súbita do patriarca Antônio Ferraz abala a Serra da Mantiqueira às vésperas da partilha milionária."
					}`,
				},
				{
					EnvelopeNumber: 2,
					Title:          "Pasta 2 – Vozes Suspeitas",
					Description:    "Depoimentos policiais gravados, contradições de álibis e histórico dos familiares e empregados presentes.",
					ContentJSON: `{
						"tipo": "depoimentos_e_vozes",
						"depoimento_1": "Rodrigo Ferraz (Filho): 'Eu estava na adega buscando outro rótulo. Meu pai e eu discutimos sobre negócios, mas jamais colocaria veneno no copo dele!'",
						"depoimento_2": "Helena Ferraz (Esposa): 'Eu servi os convidados na sala de jantar. Antônio foi ao escritório sozinho. Eu o amava, apesar de ele ameaçar me deixar sem um centavo.'",
						"depoimento_3": "Valdomiro (Capataz): 'O patrão me demitiu na frente de todos antes do jantar. Mas eu estava no galpão consertando o trator quando os gritos começaram.'",
						"depoimento_4": "Dr. Marcelo (Médico): 'Eu trouxe os calmantes habituais de Antônio. Não havia veneno na minha maleta, embora minhas receitas estivessem no bolso dele.'"
					}`,
				},
				{
					EnvelopeNumber: 3,
					Title:          "Pasta 3 – Evidências Forenses",
					Description:    "Perícia química, extratos bancários rasurados e minuta de testamento recuperada da lareira.",
					ContentJSON: `{
						"tipo": "evidencias_forenses",
						"evidencia_1": "Frasco de pesticida agrícola com resíduo de cianeto localizado na estufa de orquídeas.",
						"evidencia_2": "Fragmento queimado de testamento onde se lê claramente: '...deserdo por completo meu filho Rodrigo e nomeio...' ",
						"evidencia_3": "Comprovante de transferência bancária urgente no valor de R$ 250.000 para um agiota clandestino da capital."
					}`,
				},
			},
			DefaultSuspects: []models.Suspect{
				{
					Name:            "Rodrigo Ferraz",
					Role:            "Filho Primogênito e Herdeiro Ameaçado",
					Personality:     "Impulsivo, arrogante e viciado em apostas clandestinas",
					Alibi:           "Afirma que desceu à adega subterrânea exatamente no horário da morte",
					Secret:          "Devia 250 mil reais a criminosos com prazo de liquidação até a meia-noite",
					IsCulprit:       true,
					Tier1Alibi:      "Eu estava apenas pegando mais vinho na adega. Qualquer um na casa pode confirmar!",
					Tier2PettyCrime: "Tudo bem! Eu roubei 10 mil reais do cofre horas antes, mas não envenenei ninguém!",
					Tier3Truth:      "Ele ia me deserdar na frente de todos! Eu pinguei o cianeto na taça dele antes de subir com a garrafa!",
					CrossSuspicion:  "Helena também tinha motivos de sobra, ela sabia que seria trocada!",
				},
				{
					Name:            "Helena Ferraz",
					Role:            "Segunda Esposa e Socialite",
					Personality:     "Gélida, calculista e protetora das aparências",
					Alibi:           "Servia canapés para as visitas no salão de inverno",
					Secret:          "Mantinha uma conta secreta na Suíça com desvios da fazenda",
					IsCulprit:       false,
					Tier1Alibi:      "Passei a noite inteira coordenando o serviço do jantar com a copeira.",
					Tier2PettyCrime: "Eu transferi fundos da fazenda para uma conta pessoal, sim, para me garantir caso Antônio me expulsasse.",
					Tier3Truth:      "Eu vi Rodrigo subindo com a garrafa e as mãos tremendo, mas me calei por medo!",
					CrossSuspicion:  "Valdomiro conhece todos os venenos agrícolas da propriedade como ninguém.",
				},
				{
					Name:            "Valdomiro Cruz",
					Role:            "Capataz Veterano da Fazenda",
					Personality:     "Rústico, taciturno e ressentido pela demissão",
					Alibi:           "Estava lubrificando as correias do trator na oficina externa",
					Secret:          "Tinha as chaves da estufa de onde o veneno foi subtraído",
					IsCulprit:       false,
					Tier1Alibi:      "Fui pro galpão juntar minhas coisas depois da humilhação que passei.",
					Tier2PettyCrime: "Eu escondi duas espingardas não registradas no paiol, só isso!",
					Tier3Truth:      "A estufa ficou aberta depois das 21h porque vi alguém com capa escura saindo de lá.",
					CrossSuspicion:  "Rodrigo andou rondando os depósitos de defensivos agrícolas semana passada.",
				},
				{
					Name:            "Dr. Marcelo Sampaio",
					Role:            "Médico Pessoal de Antônio Ferraz",
					Personality:     "Polido, nervoso e dependente de sedativos",
					Alibi:           "Examinava a pressão arterial de Antônio antes do banquete",
					Secret:          "Forneceu atestado falso encobrindo a sanidade mental de Antônio",
					IsCulprit:       false,
					Tier1Alibi:      "Apenas ministrei a medicação habitual prescrita e fui ao toalete.",
					Tier2PettyCrime: "Antônio me chantageava para eu assinar laudos fraudulentos de seguros.",
					Tier3Truth:      "Quando verifiquei o pulso de Antônio na sala, já era tarde demais.",
					CrossSuspicion:  "A taça de Antônio foi servida exclusivamente por alguém da família.",
				},
			},
			DefaultClues: []models.Clue{
				{
					Location:    "Escritório de Antônio",
					Name:        "Taça de Cristal com Resíduo Amargo",
					Description: "Taça com odor característico de amêndoas amargas (cianeto) misturado ao vinho do Porto.",
				},
				{
					Location:    "Lareira do Escritório",
					Name:        "Testamento Carbonizado Parcial",
					Description: "Pedaço de pergaminho legal estipulando exclusão sumária do primogênito das terras da fazenda.",
				},
				{
					Location:    "Estufa de Orquídeas",
					Name:        "Frasco Aberto de Pesticida",
					Description: "Recipiente com pegadas recentes compatíveis com sapatos sociais de couro importado.",
				},
			},
		},
		{
			Title:          "A Morte de Sherlock Holmes",
			Description:    "O lendário detetive consultor de Baker Street foi encontrado envenenado em sua poltrona. Envelopes com telegramas da Scotland Yard, códigos matemáticos e testemunhos dos seus maiores rivais e aliados.",
			Victim:         "Sherlock Holmes",
			SolutionWeapon: "Dardo com Curare",
			SolutionMotive: "Vingança pelo desmantelamento de rede de falsificação",
			Envelopes: []models.CaseEnvelope{
				{
					EnvelopeNumber: 1,
					Title:          "Pasta 1 – Cena em Baker Street",
					Description:    "Relatório do Inspetor Lestrade e análise da lareira do 221B.",
					ContentJSON:    `{"tipo": "scotland_yard", "detalhes": "Holmes foi encontrado inerte diante do violino Stradivarius com uma marca punctiforme no pescoço."}`,
				},
				{
					EnvelopeNumber: 2,
					Title:          "Pasta 2 – Círculo de Suspeitos",
					Description:    "Depoimentos da governanta Sra. Hudson, do rival Sebastian Moran e de mensageiros.",
					ContentJSON:    `{"tipo": "depoimentos", "moran": "Eu estava no clube Diógenes jogando cartas com testemunhas."}`,
				},
				{
					EnvelopeNumber: 3,
					Title:          "Pasta 3 – Evidências Secretas",
					Description:    "Zarabatana disfarçada em bengala e anotações criptografadas.",
					ContentJSON:    `{"tipo": "evidencias", "item": "Bengala oca com mecanismo pneumático de disparo."}`,
				},
			},
		},
		{
			Title:          "O Roubo da Carga 479",
			Description:    "Noite de tempestade no cais do porto de Santos. Um contêiner militar ultrassecreto foi arrombado e o vigia noturno foi alvejado à queima-roupa.",
			Victim:         "Guarda Fonseca",
			SolutionWeapon: "Pistola Silenciada .45",
			SolutionMotive: "Suborno e queima de arquivo",
			Envelopes: []models.CaseEnvelope{
				{
					EnvelopeNumber: 1,
					Title:          "Pasta 1 – Terminal Portuário",
					Description:    "Registro de câmeras desativadas e boletim de ocorrência da Capitania dos Portos.",
					ContentJSON:    `{"tipo": "porto", "carga": "Contêiner 479 continha ligas de titânio aeroespacial."}`,
				},
			},
		},
		{
			Title:          "Caso Estrela Fria",
			Description:    "Dispositivo estelar inestimável roubado do laboratório criogênico e o astrofísico chefe executado em câmara de vácuo.",
			Victim:         "Dr. Arthur Vance",
			SolutionWeapon: "Despressurização Deliberada",
			SolutionMotive: "Espionagem corporativa multinacional",
			Envelopes: []models.CaseEnvelope{
				{
					EnvelopeNumber: 1,
					Title:          "Pasta 1 – Laboratório Criogênico",
					Description:    "Logs de acesso biométrico e laudo de asfixia hiperbárica.",
					ContentJSON:    `{"tipo": "laboratorio", "sala": "Câmara 04 sofreu despressurização manual às 03h12."}`,
				},
			},
		},
	}
}

// SeedDetectiveNightCases populates the SQLite database with the Detective Night cases and their envelopes.
func SeedDetectiveNightCases(database *sql.DB) error {
	catalog := GetDetectiveNightCatalog()

	for _, c := range catalog {
		var caseID int64
		err := database.QueryRow("SELECT id FROM cases WHERE title = ?", c.Title).Scan(&caseID)
		if err == sql.ErrNoRows {
			res, err := database.Exec(
				"INSERT INTO cases (title, description, victim, solution_weapon, solution_motive, category) VALUES (?, ?, ?, ?, ?, 'detective_night')",
				c.Title, c.Description, c.Victim, c.SolutionWeapon, c.SolutionMotive,
			)
			if err != nil {
				return err
			}
			caseID, _ = res.LastInsertId()

			// Inserir envelopes
			for _, env := range c.Envelopes {
				_, _ = database.Exec(
					"INSERT INTO case_envelopes (case_id, envelope_number, title, description, content_json) VALUES (?, ?, ?, ?, ?)",
					caseID, env.EnvelopeNumber, env.Title, env.Description, env.ContentJSON,
				)
			}

			// Inserir suspeitos padrão
			for _, s := range c.DefaultSuspects {
				culpritInt := 0
				if s.IsCulprit {
					culpritInt = 1
				}
				sRes, _ := database.Exec(
					`INSERT INTO suspects (case_id, name, role, personality, alibi, secret, is_culprit, tier1_alibi, tier2_petty_crime, tier3_truth, cross_suspicion)
					 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					caseID, s.Name, s.Role, s.Personality, s.Alibi, s.Secret, culpritInt, s.Tier1Alibi, s.Tier2PettyCrime, s.Tier3Truth, s.CrossSuspicion,
				)
				if s.IsCulprit {
					sID, _ := sRes.LastInsertId()
					_, _ = database.Exec("UPDATE cases SET solution_suspect_id = ? WHERE id = ?", sID, caseID)
				}
			}

			// Inserir pistas padrão
			for _, clue := range c.DefaultClues {
				_, _ = database.Exec(
					"INSERT INTO clues (case_id, location, name, description) VALUES (?, ?, ?, ?)",
					caseID, clue.Location, clue.Name, clue.Description,
				)
			}
		}
	}
	return nil
}

// CastCustomCharacters organizes user-submitted characters (real or fictitious) into the murder mystery using AI or procedural weaver.
func CastCustomCharacters(ctx context.Context, aiClient AIClient, database *sql.DB, caseID int64, characters []models.CustomCharacter) error {
	if len(characters) == 0 {
		return nil
	}

	telemetry.Info("game", "", fmt.Sprintf("Iniciando organização de %d personagens customizados para o caso %d", len(characters), caseID), nil)

	// Se houver IA configurada, podemos pedir para a IA harmonizar os personagens com o caso
	if aiClient != nil {
		prompt := fmt.Sprintf(
			"Você é o diretor narrativo do jogo de investigação Detective Night. Temos o caso ID %d. "+
				"Organize os seguintes personagens (reais ou fictícios) no caso criminal, definindo quem é cúmplice, quem é suspeito inocente e quem é o verdadeiro assassino. "+
				"Retorne APENAS um JSON array de objetos no formato: [{\"name\":\"...\", \"role\":\"...\", \"personality\":\"...\", \"alibi\":\"...\", \"secret\":\"...\", \"is_culprit\": false}]. "+
				"Lista de personagens:\n", caseID,
		)
		for _, c := range characters {
			prompt += fmt.Sprintf("- Nome: %s, Notas: %s, Real: %v\n", c.Name, c.Notes, c.IsReal)
		}

		resp, err := aiClient.GenerateResponse(ctx, prompt)
		if err == nil && strings.Contains(resp, "[") {
			var parsedSuspects []models.Suspect
			start := strings.Index(resp, "[")
			end := strings.LastIndex(resp, "]")
			if start >= 0 && end > start {
				jsonStr := resp[start : end+1]
				if err := json.Unmarshal([]byte(jsonStr), &parsedSuspects); err == nil && len(parsedSuspects) > 0 {
					return applyCustomSuspectsToCase(database, caseID, parsedSuspects)
				}
			}
		}
	}

	// Fallback procedural se a IA não responder ou não estiver configurada
	proceduralSuspects := make([]models.Suspect, len(characters))
	for i, c := range characters {
		isCulprit := (i == 0) // primeiro é o culpado no fallback
		role := "Convidado de Honra"
		if c.Notes != "" {
			role = c.Notes
		}
		proceduralSuspects[i] = models.Suspect{
			Name:            c.Name,
			Role:            role,
			Personality:     "Enigmático e observador",
			Alibi:           "Estava em aposentos privados durante o incidente",
			Secret:          fmt.Sprintf("Ocultava interesses financeiros e cartas secretas com a vítima"),
			IsCulprit:       isCulprit,
			Tier1Alibi:      "Eu não estava perto do local do crime, diversos presentes me viram!",
			Tier2PettyCrime: "Admito que mexi nos arquivos confidenciais antes do ocorrido.",
			Tier3Truth:      "As coisas fugiram do controle e o crime aconteceu!",
			CrossSuspicion:  "Todos os presentes têm algo a esconder nesta noite.",
		}
	}

	return applyCustomSuspectsToCase(database, caseID, proceduralSuspects)
}

func applyCustomSuspectsToCase(database *sql.DB, caseID int64, suspects []models.Suspect) error {
	// Limpar suspeitos anteriores do caso para aplicar a nova escalação
	_, _ = database.Exec("DELETE FROM suspects WHERE case_id = ?", caseID)

	var culpritID int64
	for i, s := range suspects {
		culpritInt := 0
		if s.IsCulprit {
			culpritInt = 1
		}
		res, err := database.Exec(
			`INSERT INTO suspects (case_id, name, role, personality, alibi, secret, is_culprit, tier1_alibi, tier2_petty_crime, tier3_truth, cross_suspicion)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			caseID, s.Name, s.Role, s.Personality, s.Alibi, s.Secret, culpritInt, s.Tier1Alibi, s.Tier2PettyCrime, s.Tier3Truth, s.CrossSuspicion,
		)
		if err != nil {
			return err
		}
		if s.IsCulprit {
			culpritID, _ = res.LastInsertId()
		} else if i == 0 && culpritID == 0 {
			culpritID, _ = res.LastInsertId()
		}
	}

	if culpritID > 0 {
		_, _ = database.Exec("UPDATE cases SET solution_suspect_id = ? WHERE id = ?", culpritID, caseID)
	}

	return nil
}

// GetCaseEnvelopes retrieves the three Detective Night envelopes for a given case.
func GetCaseEnvelopes(database *sql.DB, caseID int64) ([]models.CaseEnvelope, error) {
	rows, err := database.Query(
		"SELECT id, case_id, envelope_number, title, description, content_json FROM case_envelopes WHERE case_id = ? ORDER BY envelope_number ASC",
		caseID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.CaseEnvelope
	for rows.Next() {
		var env models.CaseEnvelope
		if err := rows.Scan(&env.ID, &env.CaseID, &env.EnvelopeNumber, &env.Title, &env.Description, &env.ContentJSON); err != nil {
			return nil, err
		}
		if env.ContentJSON != "" {
			var m map[string]interface{}
			if err := json.Unmarshal([]byte(env.ContentJSON), &m); err == nil {
				env.Items = m
			}
		}
		list = append(list, env)
	}
	return list, nil
}
