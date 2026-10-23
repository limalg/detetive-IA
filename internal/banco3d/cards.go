package banco3d

import (
	"errors"
	"strings"
	"sync"
)

// CardType defines the deck category (CHANCE or CHEST).
type CardType string

const (
	CardTypeChance CardType = "CHANCE"
	CardTypeChest  CardType = "CHEST"
)

// CardAction defines the mechanical effect executed when the card is drawn.
type CardAction string

const (
	CardActionMoney              CardAction = "MONEY"
	CardActionMovePos            CardAction = "MOVE_POS"
	CardActionMoveRelative       CardAction = "MOVE_RELATIVE"
	CardActionGoToJail           CardAction = "GO_TO_JAIL"
	CardActionGetOutOfJail       CardAction = "GET_OUT_OF_JAIL"
	CardActionRepairs            CardAction = "REPAIRS"
	CardActionCollectFromPlayers CardAction = "COLLECT_FROM_PLAYERS"
	CardActionPayToPlayers       CardAction = "PAY_TO_PLAYERS"
	CardActionEarthquake         CardAction = "EARTHQUAKE"
	CardActionFlood              CardAction = "FLOOD"
	CardActionFire               CardAction = "FIRE"
	CardActionBlackout           CardAction = "BLACKOUT"
	CardActionStrike             CardAction = "STRIKE"
	CardActionTaxSurprise        CardAction = "TAX_SURPRISE"
	CardActionInheritance        CardAction = "INHERITANCE"
	CardActionLottery            CardAction = "LOTTERY"
	CardActionFreeUpgrade        CardAction = "FREE_UPGRADE"
	CardActionFestival           CardAction = "FESTIVAL"
	CardActionSanitaryInspection CardAction = "SANITARY_INSPECTION"
	CardActionRealEstateCrisis   CardAction = "REAL_ESTATE_CRISIS"
)

// Card represents a thematic Chance or Community Chest card.
type Card struct {
	ID        string     `json:"id"`
	Deck      CardType   `json:"deck"`
	Type      CardType   `json:"type"` // Alias for Deck compatibility
	Title     string     `json:"title"`
	Text      string     `json:"text"`
	Effect    CardAction `json:"effect"`
	Action    CardAction `json:"action"` // Alias for Effect compatibility
	ImageKind string     `json:"image_kind,omitempty"`
	Amount    int        `json:"amount,omitempty"`
	Value     int        `json:"value,omitempty"` // Alias for Amount compatibility
	HouseCost int        `json:"house_cost,omitempty"`
	HotelCost int        `json:"hotel_cost,omitempty"`
	TargetPos int        `json:"target_pos,omitempty"`
}

// CardResult describes what happened when a card was resolved.
type CardResult struct {
	Card          Card   `json:"card"`
	PlayerID      string `json:"player_id"`
	MoneyDelta    int    `json:"money_delta"`
	NewPosition   int    `json:"new_position"`
	SalaryPaid    int    `json:"salary_paid,omitempty"`
	ImageKind     string `json:"image_kind,omitempty"`
	DestroyedTile int    `json:"destroyed_tile,omitempty"`
	DestroyedWhat string `json:"destroyed_what,omitempty"` // "house" | "hotel"
	WentToJail    bool   `json:"went_to_jail"`
	HasFreeCard   bool   `json:"has_free_card"`
}

// CityDeck holds thematic cards for a specific municipality.
type CityDeck struct {
	City   string `json:"city"`
	Chance []Card `json:"chance"`
	Chest  []Card `json:"chest"`
}

var cityCardCatalogs = map[string]CityDeck{
	"curitiba": {
		City: "Curitiba",
		Chance: []Card{
			{ID: "cwb-c1", Deck: CardTypeChance, Type: CardTypeChance, Title: "Eixo Ligeirinho", Text: "Inauguração da nova estação tubo expresso. Seus imóveis valorizaram: receba R$ 150.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 150, Value: 150, ImageKind: "lottery"},
			{ID: "cwb-c2", Deck: CardTypeChance, Type: CardTypeChance, Title: "Jardim Botânico", Text: "Prêmio Municipal de Sustentabilidade e Turismo Verde: receba R$ 100.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 100, Value: 100, ImageKind: "festival"},
			{ID: "cwb-c3", Deck: CardTypeChance, Type: CardTypeChance, Title: "Passeio Linha Turismo", Text: "Avançar até o Ponto de Partida e recolher bônus.", Effect: CardActionMovePos, Action: CardActionMovePos, TargetPos: 0, ImageKind: "strike"},
			{ID: "cwb-c4", Deck: CardTypeChance, Type: CardTypeChance, Title: "Operação Inverno", Text: "Reparos térmicos em telhados após geada: pague R$ 25 por casa e R$ 100 por hotel.", Effect: CardActionRepairs, Action: CardActionRepairs, HouseCost: 25, HotelCost: 100, Amount: 25, Value: 25, ImageKind: "flood"},
			{ID: "cwb-c5", Deck: CardTypeChance, Type: CardTypeChance, Title: "Radar na Linha Verde", Text: "Infração de trânsito em faixa exclusiva. Pague R$ 50 de multa.", Effect: CardActionMoney, Action: CardActionMoney, Amount: -50, Value: -50, ImageKind: "inspection"},
			{ID: "cwb-c6", Deck: CardTypeChance, Type: CardTypeChance, Title: "Festival de Teatro", Text: "Temporada de espetáculos com ingressos esgotados: receba R$ 50 de cada jogador.", Effect: CardActionCollectFromPlayers, Action: CardActionCollectFromPlayers, Amount: 50, Value: 50, ImageKind: "festival"},
			{ID: "cwb-c7", Deck: CardTypeChance, Type: CardTypeChance, Title: "Vá para a Prisão", Text: "Irregularidades em licitação no Centro Cívico. Vá direto para a Prisão sem passar pelo GO.", Effect: CardActionGoToJail, Action: CardActionGoToJail, ImageKind: "inspection"},
			{ID: "cwb-c8", Deck: CardTypeChance, Type: CardTypeChance, Title: "Terremoto no Batel", Text: "Um abalo sísmico atingiu a região: destruiu 1 casa ou hotel em sua propriedade!", Effect: CardActionEarthquake, Action: CardActionEarthquake, ImageKind: "earthquake"},
			{ID: "cwb-c9", Deck: CardTypeChance, Type: CardTypeChance, Title: "Enchente do Rio Belém", Text: "Forte temporal inundou ruas centrais: pague R$ 30 por casa e R$ 120 por hotel.", Effect: CardActionFlood, Action: CardActionFlood, HouseCost: 30, HotelCost: 120, ImageKind: "flood"},
			{ID: "cwb-c10", Deck: CardTypeChance, Type: CardTypeChance, Title: "Incêndio em Depósito Histórico", Text: "Chamas atingiram instalações: 1 casa destruída ou pague R$ 100 de franquia de seguro.", Effect: CardActionFire, Action: CardActionFire, Amount: -100, ImageKind: "fire"},
			{ID: "cwb-c11", Deck: CardTypeChance, Type: CardTypeChance, Title: "Apagão no Sistema Elétrico", Text: "Subestação de energia sofreu pane total: você perde a próxima rodada!", Effect: CardActionBlackout, Action: CardActionBlackout, ImageKind: "blackout"},
			{ID: "cwb-c12", Deck: CardTypeChance, Type: CardTypeChance, Title: "Greve Geral de Ônibus", Text: "Paralisação geral dos biarticulados: volte 3 casas.", Effect: CardActionStrike, Action: CardActionStrike, Amount: -3, ImageKind: "strike"},
			{ID: "cwb-c13", Deck: CardTypeChance, Type: CardTypeChance, Title: "Crise Imobiliária Regional", Text: "Desaceleração econômica afeta inquilinos: pague R$ 100 em taxa de estabilização.", Effect: CardActionRealEstateCrisis, Action: CardActionRealEstateCrisis, Amount: -100, ImageKind: "crisis"},
		},
		Chest: []Card{
			{ID: "cwb-k1", Deck: CardTypeChest, Type: CardTypeChest, Title: "Calçadão da Rua XV", Text: "Receita recorde com quiosques de flores e café: receba R$ 200.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 200, Value: 200, ImageKind: "inheritance"},
			{ID: "cwb-k2", Deck: CardTypeChest, Type: CardTypeChest, Title: "Taxa de Saneamento", Text: "Taxa extraordinária de conservação urbana de Curitiba. Pague R$ 100.", Effect: CardActionMoney, Action: CardActionMoney, Amount: -100, Value: -100, ImageKind: "crisis"},
			{ID: "cwb-k3", Deck: CardTypeChest, Type: CardTypeChest, Title: "Habeas Corpus", Text: "Saída Livre da Prisão! Guarde esta carta para quando precisar.", Effect: CardActionGetOutOfJail, Action: CardActionGetOutOfJail, ImageKind: "lottery"},
			{ID: "cwb-k4", Deck: CardTypeChest, Type: CardTypeChest, Title: "Largo da Ordem", Text: "Sucesso de vendas na feira de artesanato e gastronomia: receba R$ 100.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 100, Value: 100, ImageKind: "festival"},
			{ID: "cwb-k5", Deck: CardTypeChest, Type: CardTypeChest, Title: "Defesa Civil", Text: "Vistoria preventiva em encostas de parques: pague R$ 50.", Effect: CardActionMoney, Action: CardActionMoney, Amount: -50, Value: -50, ImageKind: "inspection"},
			{ID: "cwb-k6", Deck: CardTypeChest, Type: CardTypeChest, Title: "Vá para a Prisão", Text: "Conduzido pelas autoridades municipais à carceragem.", Effect: CardActionGoToJail, Action: CardActionGoToJail, ImageKind: "inspection"},
			{ID: "cwb-k7", Deck: CardTypeChest, Type: CardTypeChest, Title: "Prêmio da Loteria da Caixa", Text: "Seu bilhete foi sorteado na loteria estadual! Receba R$ 250.", Effect: CardActionLottery, Action: CardActionLottery, Amount: 250, Value: 250, ImageKind: "lottery"},
			{ID: "cwb-k8", Deck: CardTypeChest, Type: CardTypeChest, Title: "Herança de Família Tradicional", Text: "Partilha de bens de inventário concluída: receba R$ 200.", Effect: CardActionInheritance, Action: CardActionInheritance, Amount: 200, Value: 200, ImageKind: "inheritance"},
			{ID: "cwb-k9", Deck: CardTypeChest, Type: CardTypeChest, Title: "Reforma Urbana Gratuita", Text: "Programa municipal de revitalização de fachadas: ganhe 1 casa grátis!", Effect: CardActionFreeUpgrade, Action: CardActionFreeUpgrade, ImageKind: "free_upgrade"},
			{ID: "cwb-k10", Deck: CardTypeChest, Type: CardTypeChest, Title: "Festival Gastronômico de Santa Felicidade", Text: "Sucesso estrondoso de público: receba R$ 50 de cada jogador.", Effect: CardActionFestival, Action: CardActionFestival, Amount: 50, Value: 50, ImageKind: "festival"},
			{ID: "cwb-k11", Deck: CardTypeChest, Type: CardTypeChest, Title: "Inspeção Sanitária Rigorosa", Text: "Fiscalização encontrou irregularidades em alvará: pague R$ 80 de multa.", Effect: CardActionSanitaryInspection, Action: CardActionSanitaryInspection, Amount: -80, Value: -80, ImageKind: "inspection"},
			{ID: "cwb-k12", Deck: CardTypeChest, Type: CardTypeChest, Title: "Taxa de Asfalto Emergencial", Text: "Recapeamento asfáltico extraordinário: pague R$ 150 ao município.", Effect: CardActionTaxSurprise, Action: CardActionTaxSurprise, Amount: -150, Value: -150, ImageKind: "crisis"},
		},
	},
	"sao paulo": {
		City: "São Paulo",
		Chance: []Card{
			{ID: "sp-c1", Deck: CardTypeChance, Type: CardTypeChance, Title: "Faria Lima Bônus", Text: "Megaoperação de M&A fechada na Faria Lima. Receba bônus de R$ 200.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 200, Value: 200},
			{ID: "sp-c2", Deck: CardTypeChance, Type: CardTypeChance, Title: "Paulista Aberta", Text: "Público recorde no domingo da Paulista gera lucros no comércio: receba R$ 100.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 100, Value: 100},
			{ID: "sp-c3", Deck: CardTypeChance, Type: CardTypeChance, Title: "Multa de Rodízio", Text: "Descumprimento do rodízio veicular na Marginal. Pague R$ 50.", Effect: CardActionMoney, Action: CardActionMoney, Amount: -50, Value: -50},
			{ID: "sp-c4", Deck: CardTypeChance, Type: CardTypeChance, Title: "Vá para a Prisão", Text: "Sonegação fiscal na alfândega metropolitana. Vá direto para a Prisão.", Effect: CardActionGoToJail, Action: CardActionGoToJail},
			{ID: "sp-c5", Deck: CardTypeChance, Type: CardTypeChance, Title: "Virada Cultural", Text: "Evento patrocinado em seus estabelecimentos: receba R$ 50 de cada jogador.", Effect: CardActionCollectFromPlayers, Action: CardActionCollectFromPlayers, Amount: 50, Value: 50},
			{ID: "sp-c6", Deck: CardTypeChance, Type: CardTypeChance, Title: "Retrofit nos Jardins", Text: "Reformas de fachada em prédios históricos: pague R$ 25 por casa e R$ 100 por hotel.", Effect: CardActionRepairs, Action: CardActionRepairs, HouseCost: 25, HotelCost: 100, Amount: 25, Value: 25},
		},
		Chest: []Card{
			{ID: "sp-k1", Deck: CardTypeChest, Type: CardTypeChest, Title: "Nota Fiscal Paulista", Text: "Créditos acumulados do Tesouro Estadual! Receba R$ 150 do governo.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 150, Value: 150},
			{ID: "sp-k2", Deck: CardTypeChest, Type: CardTypeChest, Title: "IPTU Central", Text: "Revisão na planta genérica de valores do IPTU da capital: pague R$ 100.", Effect: CardActionMoney, Action: CardActionMoney, Amount: -100, Value: -100},
			{ID: "sp-k3", Deck: CardTypeChest, Type: CardTypeChest, Title: "Habeas Corpus", Text: "Liminar concedida no Tribunal de Justiça de São Paulo. Saída Livre da Prisão.", Effect: CardActionGetOutOfJail, Action: CardActionGetOutOfJail},
			{ID: "sp-k4", Deck: CardTypeChest, Type: CardTypeChest, Title: "Bovespa em Alta", Text: "Dividendos de fundos imobiliários depositados em sua conta: receba R$ 100.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 100, Value: 100},
			{ID: "sp-k5", Deck: CardTypeChest, Type: CardTypeChest, Title: "Seguro Condomínio", Text: "Renovação obrigatória de apólice de incêndio predial: pague R$ 50.", Effect: CardActionMoney, Action: CardActionMoney, Amount: -50, Value: -50},
			{ID: "sp-k6", Deck: CardTypeChest, Type: CardTypeChest, Title: "Vá para a Prisão", Text: "Operação da Polícia Civil no centro financeiro.", Effect: CardActionGoToJail, Action: CardActionGoToJail},
		},
	},
	"rio de janeiro": {
		City: "Rio de Janeiro",
		Chance: []Card{
			{ID: "rio-c1", Deck: CardTypeChance, Type: CardTypeChance, Title: "Verão Copacabana", Text: "Ocupação hoteleira atinge 100% no Réveillon e Carnaval! Receba R$ 200.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 200, Value: 200},
			{ID: "rio-c2", Deck: CardTypeChance, Type: CardTypeChance, Title: "Porto Maravilha", Text: "Revitalização da zona portuária valoriza seus investimentos: receba R$ 150.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 150, Value: 150},
			{ID: "rio-c3", Deck: CardTypeChance, Type: CardTypeChance, Title: "Ressaca no Leblon", Text: "Danos de água salgada e maresia exigem manutenção: pague R$ 100.", Effect: CardActionMoney, Action: CardActionMoney, Amount: -100, Value: -100},
			{ID: "rio-c4", Deck: CardTypeChance, Type: CardTypeChance, Title: "Vá para a Prisão", Text: "Embargo ambiental por obra irregular. Vá para a Prisão.", Effect: CardActionGoToJail, Action: CardActionGoToJail},
			{ID: "rio-c5", Deck: CardTypeChance, Type: CardTypeChance, Title: "Passeio de Bondinho", Text: "Avance até a casa de Partida (Saída) e receba seus R$ 200.", Effect: CardActionMovePos, Action: CardActionMovePos, TargetPos: 0},
			{ID: "rio-c6", Deck: CardTypeChance, Type: CardTypeChance, Title: "Almoço da Diretoria", Text: "Almoço tradicional com sócios: receba R$ 50 de cada jogador.", Effect: CardActionCollectFromPlayers, Action: CardActionCollectFromPlayers, Amount: 50, Value: 50},
		},
		Chest: []Card{
			{ID: "rio-k1", Deck: CardTypeChest, Type: CardTypeChest, Title: "Royalties do Petróleo", Text: "Divisão de dividendos de exploração da Bacia de Santos: receba R$ 200.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 200, Value: 200},
			{ID: "rio-k2", Deck: CardTypeChest, Type: CardTypeChest, Title: "Taxa de Preservação", Text: "Contribuição obrigatória para a Floresta da Tijuca: pague R$ 50.", Effect: CardActionMoney, Action: CardActionMoney, Amount: -50, Value: -50},
			{ID: "rio-k3", Deck: CardTypeChest, Type: CardTypeChest, Title: "Habeas Corpus", Text: "Decisão favorável de desembargador: Saída Livre da Prisão.", Effect: CardActionGetOutOfJail, Action: CardActionGetOutOfJail},
			{ID: "rio-k4", Deck: CardTypeChest, Type: CardTypeChest, Title: "Pedágio Linha Amarela", Text: "Tarifas de frotas comerciais: pague R$ 50.", Effect: CardActionMoney, Action: CardActionMoney, Amount: -50, Value: -50},
			{ID: "rio-k5", Deck: CardTypeChest, Type: CardTypeChest, Title: "Exposição no MAM", Text: "Patrocínio cultural aprovado pela Lei Municipal de Incentivo: receba R$ 100.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 100, Value: 100},
			{ID: "rio-k6", Deck: CardTypeChest, Type: CardTypeChest, Title: "Vá para a Prisão", Text: "Cumprimento de mandado judicial.", Effect: CardActionGoToJail, Action: CardActionGoToJail},
		},
	},
}

// Fallback generic deck for any city
var genericDeck = CityDeck{
	City: "Metrópole",
	Chance: []Card{
		{ID: "gen-c1", Deck: CardTypeChance, Type: CardTypeChance, Title: "Especulação Imobiliária", Text: "Plano diretor da cidade aprovou novos índices. Receba R$ 150.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 150, Value: 150, ImageKind: "lottery"},
		{ID: "gen-c2", Deck: CardTypeChance, Type: CardTypeChance, Title: "Avançar até o GO", Text: "Avance imediatamente até a Saída e recolha R$ 200.", Effect: CardActionMovePos, Action: CardActionMovePos, TargetPos: 0, ImageKind: "strike"},
		{ID: "gen-c3", Deck: CardTypeChance, Type: CardTypeChance, Title: "Multa de Obras", Text: "Falta de alvará provisório de construção. Pague R$ 50 ao banco.", Effect: CardActionMoney, Action: CardActionMoney, Amount: -50, Value: -50, ImageKind: "inspection"},
		{ID: "gen-c4", Deck: CardTypeChance, Type: CardTypeChance, Title: "Vá para a Prisão", Text: "Investigação fiscal detectou desvios. Vá direto para a Prisão.", Effect: CardActionGoToJail, Action: CardActionGoToJail, ImageKind: "inspection"},
		{ID: "gen-c5", Deck: CardTypeChance, Type: CardTypeChance, Title: "Reforma de Infraestrutura", Text: "Pague taxa de melhoria: R$ 25 por cada casa e R$ 100 por cada hotel.", Effect: CardActionRepairs, Action: CardActionRepairs, HouseCost: 25, HotelCost: 100, Amount: 25, Value: 25, ImageKind: "flood"},
		{ID: "gen-c6", Deck: CardTypeChance, Type: CardTypeChance, Title: "Rateio dos Acionistas", Text: "Todos os concorrentes pagam dividendos a você: receba R$ 50 de cada um.", Effect: CardActionCollectFromPlayers, Action: CardActionCollectFromPlayers, Amount: 50, Value: 50, ImageKind: "festival"},
		{ID: "gen-c7", Deck: CardTypeChance, Type: CardTypeChance, Title: "Terremoto Metropolitano", Text: "Forte abalo estrutural derrubou 1 casa ou hotel de sua propriedade!", Effect: CardActionEarthquake, Action: CardActionEarthquake, ImageKind: "earthquake"},
		{ID: "gen-c8", Deck: CardTypeChance, Type: CardTypeChance, Title: "Inundação Urbana", Text: "Enchente atingiu bairros baixos: pague R$ 30 por casa e R$ 120 por hotel.", Effect: CardActionFlood, Action: CardActionFlood, HouseCost: 30, HotelCost: 120, ImageKind: "flood"},
		{ID: "gen-c9", Deck: CardTypeChance, Type: CardTypeChance, Title: "Incêndio em Galpão", Text: "Chamas danificaram imóveis: 1 casa destruída ou pague R$ 100 de franquia.", Effect: CardActionFire, Action: CardActionFire, Amount: -100, ImageKind: "fire"},
		{ID: "gen-c10", Deck: CardTypeChance, Type: CardTypeChance, Title: "Blecaute Geral", Text: "Falta generalizada de luz paralisa operações: perca a próxima rodada!", Effect: CardActionBlackout, Action: CardActionBlackout, ImageKind: "blackout"},
		{ID: "gen-c11", Deck: CardTypeChance, Type: CardTypeChance, Title: "Greve de Transportes", Text: "Ruas bloqueadas por manifestação: recue 3 casas.", Effect: CardActionStrike, Action: CardActionStrike, Amount: -3, ImageKind: "strike"},
		{ID: "gen-c12", Deck: CardTypeChance, Type: CardTypeChance, Title: "Crise Imobiliária", Text: "Inquilinos renegociam aluguel em baixa: pague taxa emergencial de R$ 100.", Effect: CardActionRealEstateCrisis, Action: CardActionRealEstateCrisis, Amount: -100, ImageKind: "crisis"},
	},
	Chest: []Card{
		{ID: "gen-k1", Deck: CardTypeChest, Type: CardTypeChest, Title: "Erro do Banco a seu Favor", Text: "Auditoria contábil estornou taxas indevidas: receba R$ 200.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 200, Value: 200, ImageKind: "lottery"},
		{ID: "gen-k2", Deck: CardTypeChest, Type: CardTypeChest, Title: "IPTU e Taxas Municipais", Text: "Carnê de impostos municipais vencido: pague R$ 100.", Effect: CardActionMoney, Action: CardActionMoney, Amount: -100, Value: -100, ImageKind: "crisis"},
		{ID: "gen-k3", Deck: CardTypeChest, Type: CardTypeChest, Title: "Habeas Corpus", Text: "Carta de Saída Livre da Prisão. Guarde-a ou use quando detido.", Effect: CardActionGetOutOfJail, Action: CardActionGetOutOfJail, ImageKind: "lottery"},
		{ID: "gen-k4", Deck: CardTypeChest, Type: CardTypeChest, Title: "Venda de Terreno Periférico", Text: "Loteamento comercial vendido com ágio: receba R$ 100.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 100, Value: 100, ImageKind: "inheritance"},
		{ID: "gen-k5", Deck: CardTypeChest, Type: CardTypeChest, Title: "Seguro Predial", Text: "Custo anual de proteção contra riscos civis: pague R$ 50.", Effect: CardActionMoney, Action: CardActionMoney, Amount: -50, Value: -50, ImageKind: "inspection"},
		{ID: "gen-k6", Deck: CardTypeChest, Type: CardTypeChest, Title: "Vá para a Prisão", Text: "Mandado de prisão temporária expedido.", Effect: CardActionGoToJail, Action: CardActionGoToJail, ImageKind: "inspection"},
		{ID: "gen-k7", Deck: CardTypeChest, Type: CardTypeChest, Title: "Prêmio da Loteria Federal", Text: "Aposta premiada! Receba R$ 250 da Caixa Econômica.", Effect: CardActionLottery, Action: CardActionLottery, Amount: 250, Value: 250, ImageKind: "lottery"},
		{ID: "gen-k8", Deck: CardTypeChest, Type: CardTypeChest, Title: "Herança Familiar Inesperada", Text: "Parentes distantes deixaram herança em testamento: receba R$ 200.", Effect: CardActionInheritance, Action: CardActionInheritance, Amount: 200, Value: 200, ImageKind: "inheritance"},
		{ID: "gen-k9", Deck: CardTypeChest, Type: CardTypeChest, Title: "Reforma Urbana Gratuita", Text: "Subsídio governamental para modernização: ganhe 1 casa grátis!", Effect: CardActionFreeUpgrade, Action: CardActionFreeUpgrade, ImageKind: "free_upgrade"},
		{ID: "gen-k10", Deck: CardTypeChest, Type: CardTypeChest, Title: "Festival Cultural de Rua", Text: "Venda massiva de produtos autorais: receba R$ 50 de cada jogador.", Effect: CardActionFestival, Action: CardActionFestival, Amount: 50, Value: 50, ImageKind: "festival"},
		{ID: "gen-k11", Deck: CardTypeChest, Type: CardTypeChest, Title: "Inspeção Sanitária", Text: "Autuação por descarte irregular de entulho: pague R$ 80 de multa.", Effect: CardActionSanitaryInspection, Action: CardActionSanitaryInspection, Amount: -80, Value: -80, ImageKind: "inspection"},
		{ID: "gen-k12", Deck: CardTypeChest, Type: CardTypeChest, Title: "Imposto-Surpresa", Text: "Cobrança suplementar do exercício fiscal anterior: pague R$ 150.", Effect: CardActionTaxSurprise, Action: CardActionTaxSurprise, Amount: -150, Value: -150, ImageKind: "crisis"},
	},
}

// DeckManager manages card draw state and rotation per room.
type DeckManager struct {
	mu        sync.Mutex
	City      string
	chance    []Card
	chest     []Card
	chanceIdx int
	chestIdx  int
}

// NewDeckManager instantiates a deck for the specified city.
func NewDeckManager(cityName string) *DeckManager {
	deck := GetDeckForCity(cityName)
	return &DeckManager{
		City:   deck.City,
		chance: append([]Card(nil), deck.Chance...),
		chest:  append([]Card(nil), deck.Chest...),
	}
}

// DrawChance draws the next chance card in rotation.
func (dm *DeckManager) DrawChance() Card {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if len(dm.chance) == 0 {
		return genericDeck.Chance[0]
	}
	card := dm.chance[dm.chanceIdx%len(dm.chance)]
	dm.chanceIdx++
	return card
}

// DrawChest draws the next community chest card in rotation.
func (dm *DeckManager) DrawChest() Card {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if len(dm.chest) == 0 {
		return genericDeck.Chest[0]
	}
	card := dm.chest[dm.chestIdx%len(dm.chest)]
	dm.chestIdx++
	return card
}

// GetDeckForCity matches a city query to a catalog or returns generic cards.
func GetDeckForCity(query string) CityDeck {
	q := strings.ToLower(strings.TrimSpace(query))
	for key, deck := range cityCardCatalogs {
		if strings.Contains(q, key) || strings.Contains(key, q) {
			return deck
		}
	}
	return genericDeck
}

// ResolveCard applies a drawn card's effects to the player and game state.
func (g *Game) ResolveCard(playerID string, card Card) (*CardResult, error) {
	p := g.getPlayer(playerID)
	if p == nil {
		return nil, errors.New("jogador não encontrado")
	}

	result := &CardResult{
		Card:        card,
		PlayerID:    playerID,
		NewPosition: p.Position,
		ImageKind:   card.ImageKind,
	}

	amount := card.Amount
	if amount == 0 {
		amount = card.Value
	}
	effect := card.Effect
	if effect == "" {
		effect = card.Action
	}

	switch effect {
	case CardActionMoney:
		p.Money += amount
		result.MoneyDelta = amount
		if p.Money < 0 {
			g.declareBankruptcy(p)
		}
	case CardActionMovePos:
		oldPos := p.Position
		newPos := card.TargetPos
		p.Position = newPos
		result.NewPosition = newPos
		salary := g.GetSalaryAmount()
		if newPos == 0 || newPos < oldPos {
			p.Money += salary
			result.SalaryPaid += salary
			result.MoneyDelta += salary
		}
		g.resolveTileLanding(p)
	case CardActionMoveRelative:
		oldPos := p.Position
		newPos := (p.Position + amount + len(g.Board.Tiles)) % len(g.Board.Tiles)
		p.Position = newPos
		result.NewPosition = newPos
		if amount > 0 && (newPos == 0 || newPos < oldPos) {
			salary := g.GetSalaryAmount()
			p.Money += salary
			result.SalaryPaid += salary
			result.MoneyDelta += salary
		}
		g.resolveTileLanding(p)
	case CardActionGoToJail:
		p.Position = 10
		p.InJail = true
		p.IsJailed = true
		p.JailTurns = 0
		result.NewPosition = 10
		result.WentToJail = true
	case CardActionGetOutOfJail:
		p.GetOutOfJailCards++
		result.HasFreeCard = true
	case CardActionRepairs:
		houseRate := card.HouseCost
		if houseRate == 0 {
			houseRate = 25
		}
		hotelRate := card.HotelCost
		if hotelRate == 0 {
			hotelRate = 100
		}
		houses := 0
		hotels := 0
		for _, idx := range p.Properties {
			t := &g.Board.Tiles[idx]
			if t.Houses >= 5 {
				hotels++
			} else {
				houses += t.Houses
			}
		}
		cost := houses*houseRate + hotels*hotelRate
		p.Money -= cost
		result.MoneyDelta = -cost
		if p.Money < 0 {
			g.declareBankruptcy(p)
		}
	case CardActionCollectFromPlayers:
		collected := 0
		for _, other := range g.Players {
			if other.ID != p.ID && !other.IsBankrupt {
				other.Money -= amount
				collected += amount
				if other.Money < 0 {
					g.declareBankruptcy(other)
				}
			}
		}
		p.Money += collected
		result.MoneyDelta = collected
	case CardActionPayToPlayers:
		paid := 0
		for _, other := range g.Players {
			if other.ID != p.ID && !other.IsBankrupt {
				other.Money += amount
				paid += amount
			}
		}
		p.Money -= paid
		result.MoneyDelta = -paid
		if p.Money < 0 {
			g.declareBankruptcy(p)
		}
	case CardActionEarthquake:
		var targetTile *Tile
		highestPrice := -1
		for _, idx := range p.Properties {
			t := &g.Board.Tiles[idx]
			if t.Houses > 0 {
				if t.Houses >= 5 {
					targetTile = t
					break
				}
				if t.Price > highestPrice {
					highestPrice = t.Price
					targetTile = t
				}
			}
		}
		if targetTile != nil {
			result.DestroyedTile = targetTile.Index
			if targetTile.Houses >= 5 {
				targetTile.Houses = 4
				result.DestroyedWhat = "hotel"
			} else {
				targetTile.Houses--
				result.DestroyedWhat = "house"
			}
		}
	case CardActionFlood:
		houseRate := card.HouseCost
		if houseRate == 0 {
			houseRate = 30
		}
		hotelRate := card.HotelCost
		if hotelRate == 0 {
			hotelRate = 120
		}
		houses := 0
		hotels := 0
		for _, idx := range p.Properties {
			t := &g.Board.Tiles[idx]
			if t.Houses >= 5 {
				hotels++
			} else {
				houses += t.Houses
			}
		}
		cost := houses*houseRate + hotels*hotelRate
		if cost == 0 && amount != 0 {
			cost = amount
			if cost < 0 {
				cost = -cost
			}
		}
		p.Money -= cost
		result.MoneyDelta = -cost
		if p.Money < 0 {
			g.declareBankruptcy(p)
		}
	case CardActionFire:
		var targetTile *Tile
		for _, idx := range p.Properties {
			t := &g.Board.Tiles[idx]
			if t.Houses > 0 {
				targetTile = t
				break
			}
		}
		if targetTile != nil {
			result.DestroyedTile = targetTile.Index
			if targetTile.Houses >= 5 {
				targetTile.Houses = 4
				result.DestroyedWhat = "hotel"
			} else {
				targetTile.Houses--
				result.DestroyedWhat = "house"
			}
		} else {
			cost := 100
			if amount != 0 {
				cost = amount
				if cost < 0 {
					cost = -cost
				}
			}
			p.Money -= cost
			result.MoneyDelta = -cost
			if p.Money < 0 {
				g.declareBankruptcy(p)
			}
		}
	case CardActionBlackout:
		p.SkipNextTurn = true
	case CardActionStrike:
		steps := amount
		if steps == 0 {
			steps = -3
		}
		oldPos := p.Position
		newPos := (oldPos + steps + len(g.Board.Tiles)) % len(g.Board.Tiles)
		p.Position = newPos
		result.NewPosition = newPos
		g.resolveTileLanding(p)
	case CardActionTaxSurprise:
		tax := amount
		if tax == 0 {
			tax = 150
		}
		if tax > 0 {
			tax = -tax
		}
		p.Money += tax
		result.MoneyDelta = tax
		if p.Money < 0 {
			g.declareBankruptcy(p)
		}
	case CardActionInheritance:
		bonus := amount
		if bonus == 0 {
			bonus = 200
		}
		p.Money += bonus
		result.MoneyDelta = bonus
	case CardActionLottery:
		prize := amount
		if prize == 0 {
			prize = 250
		}
		p.Money += prize
		result.MoneyDelta = prize
	case CardActionFreeUpgrade:
		var upgradedTile *Tile
		for _, idx := range p.Properties {
			t := &g.Board.Tiles[idx]
			if t.Type == TileProperty && t.Houses < 5 {
				upgradedTile = t
				break
			}
		}
		if upgradedTile != nil {
			upgradedTile.Houses++
		}
	case CardActionFestival:
		fee := amount
		if fee == 0 {
			fee = 50
		}
		collected := 0
		for _, other := range g.Players {
			if other.ID != p.ID && !other.IsBankrupt {
				other.Money -= fee
				collected += fee
				if other.Money < 0 {
					g.declareBankruptcy(other)
				}
			}
		}
		p.Money += collected
		result.MoneyDelta = collected
	case CardActionSanitaryInspection:
		fine := amount
		if fine == 0 {
			fine = 80
		}
		if fine > 0 {
			fine = -fine
		}
		p.Money += fine
		result.MoneyDelta = fine
		if p.Money < 0 {
			g.declareBankruptcy(p)
		}
	case CardActionRealEstateCrisis:
		cost := amount
		if cost == 0 {
			cost = 100
		}
		if cost > 0 {
			cost = -cost
		}
		p.Money += cost
		result.MoneyDelta = cost
		if p.Money < 0 {
			g.declareBankruptcy(p)
		}
	}

	g.LastCard = &card
	return result, nil
}

// ApplyCard is an alias for ResolveCard compatibility.
func (g *Game) ApplyCard(playerID string, card Card) error {
	_, err := g.ResolveCard(playerID, card)
	return err
}
