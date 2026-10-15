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
	{
		name:        "Coronel Hector Vance",
		role:        "Veterano de Guerra & Colecionador de Armas",
		personality: "Rígido, taciturno e observador com olhar penetrante.",
		alibi:       "Estava no salão de jogos polindo peças de xadrez e fumando cachimbo.",
		secret:      "Vendeu condecorações militares e armas contrabandeadas para saldar penhoras.",
		tier1Alibi:  "Estava no salão de jogos polindo peças de xadrez e fumando cachimbo na lareira.",
		tier2Petty:  "Vendi condecorações e armas desativadas sem registro para pagar agiotas.",
		tier3Truth:  "A vítima descobriu minha desonra militar e pretendia me expor no clube dos oficiais!",
	},
	{
		name:        "Madame Genevieve Leclair",
		role:        "Antiquária & Falsificadora",
		personality: "Refinada, enigmática e atenta a detalhes materiais.",
		alibi:       "Estava na biblioteca catalogando manuscritos raros e iluminuras antigas.",
		secret:      "Autenticou artefatos forjados de alta joalheria para leilões da alta sociedade.",
		tier1Alibi:  "Estava na biblioteca catalogando manuscritos raros e iluminuras antigas até tarde.",
		tier2Petty:  "Vendi peças renascentistas forjadas e ocultei laudos falsos de autenticidade.",
		tier3Truth:  "A vítima descobriu as falsificações e pretendia me desmascarar publicamente!",
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
	{
		name:         "Navalha Pericial de Aço",
		physicalDesc: "Lâmina afiada de navalha cirúrgica com vestígios microscópicos de fibra de seda e sangue arterial",
	},
}

type themeDefinition struct {
	id                    string
	title                 string
	victim                string
	victimShort           string
	crimeScene            string
	contradictionLoc      string
	contradictionClueName string
	contradictionFormat   string
	docLoc                string
	atmosLoc              string
	atmosName             string
	atmosDesc             string
	culpritAlibi          string
	caseDescFormat        string
}

var themesMap = map[string]themeDefinition{
	"mansion": {
		id:                    "mansion",
		title:                 "O Mistério da Mansão do Vale das Sombras",
		victim:                "Sir Arthur Pendelton (Magnata das Ferrovias e Colecionador de Arte)",
		victimShort:           "Sir Arthur",
		crimeScene:            "Gabinete Privado",
		contradictionLoc:      "Portaria & Registros de Serviço",
		contradictionClueName: "Registro de Manutenção e Álibi Falso",
		contradictionFormat:   "Registro de Manutenção e Testemunho: Todo o salão de música e sua ala estiveram trancados para reparo emergencial de vazamento de gás das 21h às 04h. O álibi alegado por %s contém contradição fatal e é falso.",
		docLoc:                "Cofre Oculto da Biblioteca",
		atmosLoc:              "Jardim de Inverno",
		atmosName:             "Pegadas Úmidas e Cinzas de Charuto",
		atmosDesc:             "Rastro de calçados elegantes de gala molhados pela chuva recente, com cinzas importadas deixadas na fuga.",
		culpritAlibi:          "Afirma que passou a noite inteira lendo no salão de música com uma taça de vinho ao lado da lareira.",
		caseDescFormat:        "Na fria madrugada chuvosa, o corpo de %s foi encontrado sem vida em seu gabinete privado na Mansão das Sombras. %d pessoas presentes na propriedade foram isoladas para interrogatório.",
	},
	"train": {
		id:                    "train",
		title:                 "O Mistério do Expresso da Meia-Noite",
		victim:                "Viktor Orlov (Diplomata e Colecionador de Segredos)",
		victimShort:           "Viktor Orlov",
		crimeScene:            "Cabine de Primeira Classe (Vagão Leito A)",
		contradictionLoc:      "Registro do Condutor & Prancheta de Bordo",
		contradictionClueName: "Registro de Bordo e Álibi Falso",
		contradictionFormat:   "Registro de Bordo do Condutor: O vagão-restaurante foi evacuado e trancado às 22h30 devido a um curto-circuito na cozinha. O álibi alegado por %s contém contradição fatal e é falso.",
		docLoc:                "Mala Diplomática com Fundo Falso",
		atmosLoc:              "Plataforma de Conexão Entre Vagões",
		atmosName:             "Cinzas de Fumo Turco e Vidro Trincado",
		atmosDesc:             "Fragmentos de vidro da janela de observação com resíduos de fumo turco espalhados pela ventania dos trilhos.",
		culpritAlibi:          "Afirma que permaneceu durante toda a travessia no vagão-restaurante degustando licor com outros passageiros.",
		caseDescFormat:        "Enquanto o expresso cortava as montanhas sob nevasca densa, o corpo de %s foi encontrado sem vida em sua cabine de luxo. %d passageiros suspeitos foram retidos até a próxima estação.",
	},
	"opera": {
		id:                    "opera",
		title:                 "A Última Ária no Teatro Municipal",
		victim:                "Gabriel Fontaine (Maestro Principal e Tenor de Renome)",
		victimShort:           "Gabriel Fontaine",
		crimeScene:            "Camarim Principal do Teatro",
		contradictionLoc:      "Portaria de Artistas & Quadro de Chaves",
		contradictionClueName: "Quadro de Chaves e Álibi Falso",
		contradictionFormat:   "Quadro de Chaves e Registro da Portaria: A cabine de iluminação operou em modo automático e a chave permaneceu lacrada no cofre do contra-regra a noite inteira. O álibi alegado por %s contém contradição fatal e é falso.",
		docLoc:                "Partitura Trancada no Fosso da Orquestra",
		atmosLoc:              "Bastidores do Palco",
		atmosName:             "Pó de Maquiagem Teatral e Luva de Seda",
		atmosDesc:             "Manchas de pó de arroz de palco e uma luva solitária de seda negra com aroma de perfume francês nos bastidores.",
		culpritAlibi:          "Afirma que esteve trancado na cabine de iluminação afinando refletores durante todo o terceiro ato.",
		caseDescFormat:        "Durante os aplausos finais da ópera de gala, o corpo de %s foi descoberto estendido em seu camarim sob a luz pálida dos espelhos. %d pessoas do elenco e convidados foram detidos nos bastidores.",
	},
	"casino": {
		id:                    "casino",
		title:                 "A Dívida do Cassino Clandestino",
		victim:                "Emilio Ramos (Dono de Bancas Ilegais e Agiota do Submundo)",
		victimShort:           "Emilio Ramos",
		crimeScene:            "Sala VIP dos Fundos do Cassino",
		contradictionLoc:      "Guichê de Câmbio & Livro de Caixas",
		contradictionClueName: "Livro de Caixas e Álibi Falso",
		contradictionFormat:   "Livro de Caixas e Depoimento do Chefe de Mesa: A mesa de bacará esteve fechada para recontagem de fichas desde a meia-noite por suspeita de cartas marcadas. O álibi alegado por %s contém contradição fatal e é falso.",
		docLoc:                "Cofre Falso sob a Mesa de Roleta",
		atmosLoc:              "Beco das Docas Atrás do Cassino",
		atmosName:             "Fichas Manchadas e Marcas de Pneu",
		atmosDesc:             "Fichas de marfim de alto valor jogadas na sarjeta úmida e marcas recentes de arrancada de pneu na saída de emergência.",
		culpritAlibi:          "Afirma que esteve na mesa de bacará pública jogando à vista de todos os croupiers durante a madrugada.",
		caseDescFormat:        "Nas sombras dos fundos do clube clandestino de apostas, o corpo de %s foi encontrado com sinais claros de violência. %d frequentadores de alta periculosidade foram retidos pelos seguranças.",
	},
}

func getTheme(themeID string) themeDefinition {
	if th, ok := themesMap[themeID]; ok {
		return th
	}
	return themesMap["mansion"]
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

// WeaveCase procedurally constructs a murder mystery following the Three-Clue Rule with default mansion theme and 4 suspects.
func (w *Weaver) WeaveCase(players []room.Player) (*WeaverResult, error) {
	return w.WeaveCaseWithTheme(players, "mansion", 4)
}

// WeaveCaseWithTheme procedurally constructs a murder mystery for the chosen theme and suspect count.
func (w *Weaver) WeaveCaseWithTheme(players []room.Player, themeID string, suspectCount int) (*WeaverResult, error) {
	if suspectCount <= 0 {
		suspectCount = 4
	} else if suspectCount < 3 {
		suspectCount = 3
	} else if suspectCount > 6 {
		suspectCount = 6
	}

	theme := getTheme(themeID)

	// 1. Fill suspect pool with players and bots up to suspectCount
	suspectPool := make([]WeaverSuspect, 0, suspectCount)

	for _, p := range players {
		if len(suspectPool) >= suspectCount {
			break
		}
		persona := p.PersonaName
		if persona == "" {
			persona = p.Username
		}
		role := p.Role
		if role == "" {
			role = "Investigador Convidado"
		}
		alibi := "Afirma que permaneceu nos corredores admirando as instalações na hora fatídica."
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
	for len(suspectPool) < suspectCount && botIdx < len(noirBotPool) {
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
	culprit.Tier1Alibi = theme.culpritAlibi
	culprit.Alibi = culprit.Tier1Alibi
	culprit.Tier3Truth = fmt.Sprintf("%s descobriu tudo sobre %s e ia me arruinar perante a sociedade! Quando ele me ameaçou, perdi a cabeça e usei a %s!", theme.victimShort, motive.name, weapon.name)

	// Ensure all innocents have coherent Tier 3 truth
	for i := range suspectPool {
		if !suspectPool[i].IsCulprit {
			suspectPool[i].Tier3Truth = fmt.Sprintf("Posso ter meus pecados e segredos menores, mas sou inocente deste crime horrendo! Jamais assassinaria %s!", theme.victimShort)
		}
	}

	// Cross-dialogue social matrix: each suspect suspects another suspect by name
	for i := range suspectPool {
		otherIdx := (i + 1) % len(suspectPool)
		other := suspectPool[otherIdx]
		suspectPool[i].CrossSuspicion = fmt.Sprintf("Desconfio de %s; vi passos apressados perto da cena do crime e discussões tensas a portas fechadas.", other.Name)
	}

	contradictionText := fmt.Sprintf(theme.contradictionFormat, culprit.Name)

	victimName := theme.victim
	caseTitle := theme.title
	caseDesc := fmt.Sprintf(theme.caseDescFormat, victimName, len(suspectPool))

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
			Location:    fmt.Sprintf("Cena do Crime (%s)", theme.crimeScene),
			Name:        weapon.name,
			Description: weapon.physicalDesc + " encontrada com resquícios periciais evidentes.",
		},
		// Clue 2: Alibi Contradiction exposing the killer's lie
		{
			CaseID:      caseID,
			Location:    theme.contradictionLoc,
			Name:        theme.contradictionClueName,
			Description: contradictionText,
		},
		// Clue 3: Documentary evidence proving motive
		{
			CaseID:      caseID,
			Location:    theme.docLoc,
			Name:        "Documento: " + motive.docName,
			Description: motive.docDesc + fmt.Sprintf(" diretamente conectado a %s.", savedCulprit.Name),
		},
		// Clue 4: Circumstantial atmospheric clue
		{
			CaseID:      caseID,
			Location:    theme.atmosLoc,
			Name:        theme.atmosName,
			Description: theme.atmosDesc,
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
