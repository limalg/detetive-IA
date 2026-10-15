package game

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"

	"detetive-ia/internal/models"
	"detetive-ia/internal/room"
)

// WeaverSuspect represents a suspect woven into the case.
type WeaverSuspect struct {
	models.Suspect
	IsBot bool `json:"is_bot"`
}

// WeaverResult holds the generated mystery, solution graph, and evidence.
type WeaverResult struct {
	CaseID        int64           `json:"case_id"`
	Case          models.Case     `json:"case"`
	Culprit       WeaverSuspect   `json:"culprit"`
	Suspects      []WeaverSuspect `json:"suspects"`
	Clues         []models.Clue   `json:"clues"`
	Contradiction string          `json:"contradiction"`
}

// Weaver implements the backwards procedural mystery engine.
type Weaver struct {
	db *sql.DB
}

// NewWeaver constructs a new mystery weaver.
func NewWeaver(db *sql.DB) *Weaver {
	return &Weaver{db: db}
}

type botArchetype struct {
	name        string
	role        string
	personality string
	alibi       string
	secret      string
	tier1Alibi  string
	tier2Petty  string
	tier3Truth  string
}

var noirBotPool = []botArchetype{
	{
		name:        "Dra. Nicole Moreau",
		role:        "Médica Patologista",
		personality: "Metódica, cínica e fumante inveterada.",
		alibi:       "Estava no laboratório temporário conferindo laudos toxicológicos.",
		secret:      "Prescrevia morfina sob nomes falsos para aristocratas da cidade.",
		tier1Alibi:  "Estava no laboratório temporário conferindo laudos toxicológicos e dosagens.",
		tier2Petty:  "Prescrevia morfina e anestésicos sob nomes falsos para clientes ricos da alta sociedade.",
		tier3Truth:  "Arthur me chantageava com o livro de receitas ilegais e ia cassar minha licença médica!",
	},
	{
		name:        "Vincent Thorne",
		role:        "Jornalista Investigativo",
		personality: "Oportunista, sarcástico e atento a sussurros de bastidores.",
		alibi:       "Fiquei no saguão datilografando notas sobre o baile beneficente.",
		secret:      "Chantageava personalidades públicas com fotografias clandestinas.",
		tier1Alibi:  "Fiquei no saguão datilografando notas sobre o baile beneficente até altas horas.",
		tier2Petty:  "Chantageava personalidades públicas e políticos com fotografias clandestinas do submundo.",
		tier3Truth:  "Arthur destruiu os negativos de fotos que garantiriam minha fortuna e reputação nacional!",
	},
	{
		name:        "Barão Klaus von Berg",
		role:        "Magnata & Agiota",
		personality: "Arrogante, implacável e impaciente com devedores.",
		alibi:       "Estava na sala de conferências revisando contratos de hipotecas antigas.",
		secret:      "Financiou contrabando no cais e a vítima ameaçou denunciar às autoridades.",
		tier1Alibi:  "Estava na sala de conferências revisando contratos de hipotecas e títulos de crédito.",
		tier2Petty:  "Financiei carregamentos clandestinos no cais e opero uma mesa ilegal de câmbio negro.",
		tier3Truth:  "Arthur ameaçou entregar meus livros contábeis à receita federal e arruinar meu império!",
	},
	{
		name:        "Clarice Dubois",
		role:        "Cantora de Cabaré",
		personality: "Sedutora, melancólica e ardilosa.",
		alibi:       "Permaneci no camarim ensaiando arranjos de voz ao violino até a madrugada.",
		secret:      "Possuía cartas de amor escandalosas enviadas pela vítima anos atrás.",
		tier1Alibi:  "Permaneci no camarim ensaiando arranjos de voz ao violino até a madrugada.",
		tier2Petty:  "Extorquia amantes e guardava joias furtadas de mecenas desavisados em meu camarim.",
		tier3Truth:  "Arthur ameaçou me expulsar da cidade sem um tostão se eu não lhe devolvesse as cartas íntimas!",
	},
	{
		name:        "Barnaby Finch",
		role:        "Mordomo & Guarda-livros",
		personality: "Formal, impassível e rigorosamente detalhista.",
		alibi:       "Conferia o inventário de garrafas e cristais na adega subterrânea.",
		secret:      "Ocultava recibos contábeis comprometedores da família no forro do teto.",
		tier1Alibi:  "Conferia o inventário de garrafas e cristais na adega subterrânea da mansão.",
		tier2Petty:  "Ocultava recibos contábeis comprometedores e desviava pequenos montantes da despensa.",
		tier3Truth:  "Arthur pretendia me demitir sem pensão após quarenta anos de servidão silenciosa!",
	},
}

type weaponDefinition struct {
	name         string
	physicalDesc string
}

var weaponPool = []weaponDefinition{
	{
		name:         "Adaga Cerimonial Veneziana",
		physicalDesc: "Lâmina ornamentada com pó de madrepérola e resíduo de sangue seco no cabo",
	},
	{
		name:         "Cálice com Resíduo de Cianeto",
		physicalDesc: "Cálice de cristal com resíduo cristalino de veneno e odor amargo de amêndoas",
	},
	{
		name:         "Revólver .38 com Silenciador",
		physicalDesc: "Cápsula deflagrada de projétil .38 e pólvora queimada em tecido de veludo",
	},
	{
		name:         "Castiçal de Bronze Maciço",
		physicalDesc: "Castiçal pesado de bronze com a haste amassada por golpe violento e fios de cabelo presos",
	},
}

type motiveDefinition struct {
	name    string
	docName string
	docDesc string
}

var motivePool = []motiveDefinition{
	{
		name:    "Herança e dívidas de jogo secretas",
		docName: "Testamento Alterado e Notas Promissórias",
		docDesc: "Rascunho de testamento deserdando formalmente o herdeiro e cobranças de cassinos clandestinos",
	},
	{
		name:    "Chantagem por desvio de fundos corporativos",
		docName: "Livro Contábil Adulterado",
		docDesc: "Auditoria interna comprovando rombo milionário e ameaça iminente de denúncia criminal",
	},
	{
		name:    "Vingança por ruína e traição familiar",
		docName: "Correspondência Confidencial e Ordem de Despejo",
		docDesc: "Cartas ácidas demonstrando rancor profundo pela destruição da reputação familiar",
	},
}

// WeaveCase procedurally constructs a murder mystery following the Three-Clue Rule.
func (w *Weaver) WeaveCase(players []room.Player) (*WeaverResult, error) {
	// 1. Fill suspect pool with bots if fewer than 4 players
	suspectPool := make([]WeaverSuspect, 0, 4)

	for _, p := range players {
		persona := p.PersonaName
		if persona == "" {
			persona = p.Username
		}
		role := p.Role
		if role == "" {
			role = "Investigador Convidado"
		}
		alibi := "Afirma que permaneceu na galeria de arte admirando os quadros na hora fatídica."
		petty := "Ocultava transações financeiras irregulares e dívidas nos bastidores da alta sociedade."
		truth := "Eu estava apenas tentando salvar minhas economias de uma ruína iminente!"
		suspectPool = append(suspectPool, WeaverSuspect{
			Suspect: models.Suspect{
				Name:            persona,
				Role:            role,
				Personality:     "Observador, reservado e misterioso.",
				Alibi:           alibi,
				Secret:          p.Secret,
				Tier1Alibi:      alibi,
				Tier2PettyCrime: petty,
				Tier3Truth:      truth,
			},
			IsBot: p.IsBot,
		})
	}

	botIdx := 0
	for len(suspectPool) < 4 && botIdx < len(noirBotPool) {
		bot := noirBotPool[botIdx]
		suspectPool = append(suspectPool, WeaverSuspect{
			Suspect: models.Suspect{
				Name:            bot.name,
				Role:            bot.role,
				Personality:     bot.personality,
				Alibi:           bot.tier1Alibi,
				Secret:          bot.secret,
				Tier1Alibi:      bot.tier1Alibi,
				Tier2PettyCrime: bot.tier2Petty,
				Tier3Truth:      bot.tier3Truth,
			},
			IsBot: true,
		})
		botIdx++
	}

	// 2. Select Culprit randomly
	culpritIdx := cryptoRandInt(len(suspectPool))
	for i := range suspectPool {
		suspectPool[i].IsCulprit = (i == culpritIdx)
	}
	culprit := &suspectPool[culpritIdx]

	// 3. Select Weapon and Motive
	weapon := weaponPool[cryptoRandInt(len(weaponPool))]
	motive := motivePool[cryptoRandInt(len(motivePool))]

	// 4. Construct Contradiction for Culprit's Alibi and Narrative Tiers
	culprit.Tier1Alibi = "Afirma que passou a noite inteira lendo no salão de música com uma taça de vinho ao lado da lareira."
	culprit.Alibi = culprit.Tier1Alibi
	culprit.Tier3Truth = fmt.Sprintf("Arthur descobriu tudo sobre %s e ia me arruinar perante a sociedade! Quando ele me ameaçou, perdi a cabeça e usei a %s!", motive.name, weapon.name)

	// Ensure all innocents have coherent Tier 3 truth
	for i := range suspectPool {
		if !suspectPool[i].IsCulprit {
			suspectPool[i].Tier3Truth = "Posso ter meus pecados e segredos menores, mas sou inocente deste crime horrendo! Jamais assassinaria Sir Arthur!"
		}
	}

	// Cross-dialogue social matrix: each suspect suspects another suspect by name
	for i := range suspectPool {
		otherIdx := (i + 1) % len(suspectPool)
		other := suspectPool[otherIdx]
		suspectPool[i].CrossSuspicion = fmt.Sprintf("Desconfio de %s; vi passos apressados perto do gabinete e discussões tensas a portas fechadas.", other.Name)
	}

	contradictionText := fmt.Sprintf("Registro de Manutenção e Testemunho: Todo o salão de música e sua ala estiveram trancados para reparo emergencial de vazamento de gás das 21h às 04h. O álibi alegado por %s contém contradição fatal e é falso.", culprit.Name)

	// Victim profile
	victimName := "Arthur Pendelton (Magnata das Ferrovias e Colecionador de Arte)"
	caseTitle := "O Mistério da Mansão do Vale das Sombras"
	caseDesc := fmt.Sprintf("Na fria madrugada chuvosa, o corpo de %s foi encontrado sem vida em seu gabinete privado na Mansão das Sombras. Quatro pessoas presentes na propriedade foram isoladas para interrogatório.", victimName)

	// 5. Persist Case in SQLite
	var caseID int64
	if w.db != nil {
		res, err := w.db.Exec(`
			INSERT INTO cases (title, description, victim, solution_weapon, solution_motive)
			VALUES (?, ?, ?, ?, ?)
		`, caseTitle, caseDesc, victimName, weapon.name, motive.name)
		if err != nil {
			return nil, fmt.Errorf("failed to insert generated case: %w", err)
		}
		caseID, err = res.LastInsertId()
		if err != nil {
			return nil, err
		}
	}

	// Persist Suspects
	var savedCulprit WeaverSuspect
	savedSuspects := make([]WeaverSuspect, len(suspectPool))

	for i, s := range suspectPool {
		if w.db != nil {
			sRes, err := w.db.Exec(`
				INSERT INTO suspects (case_id, name, role, personality, alibi, secret, is_culprit, tier1_alibi, tier2_petty_crime, tier3_truth, cross_suspicion)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, caseID, s.Name, s.Role, s.Personality, s.Alibi, s.Secret, s.IsCulprit, s.Tier1Alibi, s.Tier2PettyCrime, s.Tier3Truth, s.CrossSuspicion)
			if err != nil {
				return nil, fmt.Errorf("failed to insert suspect %s: %w", s.Name, err)
			}
			s.ID, _ = sRes.LastInsertId()
			s.CaseID = caseID
		}
		savedSuspects[i] = s
		if s.IsCulprit {
			savedCulprit = s
			if w.db != nil {
				_, _ = w.db.Exec(`UPDATE cases SET solution_suspect_id = ? WHERE id = ?`, s.ID, caseID)
			}
		}
	}

	// 6. Generate Three-Clue Rule Evidence
	cluesToInsert := []models.Clue{
		// Clue 1: Physical evidence pointing to the weapon
		{
			CaseID:      caseID,
			Location:    "Cena do Crime (Gabinete)",
			Name:        weapon.name,
			Description: weapon.physicalDesc + " encontrada com resquícios periciais evidentes.",
		},
		// Clue 2: Alibi Contradiction exposing the killer's lie
		{
			CaseID:      caseID,
			Location:    "Portaria & Registros de Serviço",
			Name:        "Registro de Manutenção e Álibi Falso",
			Description: contradictionText,
		},
		// Clue 3: Documentary evidence proving motive
		{
			CaseID:      caseID,
			Location:    "Cofre Oculto da Biblioteca",
			Name:        "Documento: " + motive.docName,
			Description: motive.docDesc + fmt.Sprintf(" diretamente conectado a %s.", savedCulprit.Name),
		},
		// Clue 4: Circumstantial atmospheric clue
		{
			CaseID:      caseID,
			Location:    "Jardim de Inverno",
			Name:        "Pegadas Úmidas e Cinzas de Charuto",
			Description: "Rastro de calçados elegantes de gala molhados pela chuva recente, com cinzas importadas deixadas na fuga.",
		},
	}

	savedClues := make([]models.Clue, len(cluesToInsert))
	for i, c := range cluesToInsert {
		if w.db != nil {
			cRes, err := w.db.Exec(`
				INSERT INTO clues (case_id, location, name, description)
				VALUES (?, ?, ?, ?)
			`, caseID, c.Location, c.Name, c.Description)
			if err != nil {
				return nil, fmt.Errorf("failed to insert clue: %w", err)
			}
			c.ID, _ = cRes.LastInsertId()
		}
		savedClues[i] = c
	}

	return &WeaverResult{
		CaseID: caseID,
		Case: models.Case{
			ID:                caseID,
			Title:             caseTitle,
			Description:       caseDesc,
			Victim:            victimName,
			SolutionSuspectID: savedCulprit.ID,
			SolutionWeapon:    weapon.name,
			SolutionMotive:    motive.name,
		},
		Culprit:       savedCulprit,
		Suspects:      savedSuspects,
		Clues:         savedClues,
		Contradiction: contradictionText,
	}, nil
}

func cryptoRandInt(max int) int {
	if max <= 1 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0
	}
	return int(n.Int64())
}
