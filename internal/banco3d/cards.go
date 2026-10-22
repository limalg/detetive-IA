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
	Amount    int        `json:"amount,omitempty"`
	Value     int        `json:"value,omitempty"` // Alias for Amount compatibility
	HouseCost int        `json:"house_cost,omitempty"`
	HotelCost int        `json:"hotel_cost,omitempty"`
	TargetPos int        `json:"target_pos,omitempty"`
}

// CardResult describes what happened when a card was resolved.
type CardResult struct {
	Card        Card   `json:"card"`
	PlayerID    string `json:"player_id"`
	MoneyDelta  int    `json:"money_delta"`
	NewPosition int    `json:"new_position"`
	WentToJail  bool   `json:"went_to_jail"`
	HasFreeCard bool   `json:"has_free_card"`
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
			{ID: "cwb-c1", Deck: CardTypeChance, Type: CardTypeChance, Title: "Eixo Ligeirinho", Text: "Inauguração da nova estação tubo expresso. Seus imóveis valorizaram: receba R$ 150.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 150, Value: 150},
			{ID: "cwb-c2", Deck: CardTypeChance, Type: CardTypeChance, Title: "Jardim Botânico", Text: "Prêmio Municipal de Sustentabilidade e Turismo Verde: receba R$ 100.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 100, Value: 100},
			{ID: "cwb-c3", Deck: CardTypeChance, Type: CardTypeChance, Title: "Passeio Linha Turismo", Text: "Avançar até o Ponto de Partida e recolher bônus.", Effect: CardActionMovePos, Action: CardActionMovePos, TargetPos: 0},
			{ID: "cwb-c4", Deck: CardTypeChance, Type: CardTypeChance, Title: "Operação Inverno", Text: "Reparos térmicos em telhados após geada: pague R$ 25 por casa e R$ 100 por hotel.", Effect: CardActionRepairs, Action: CardActionRepairs, HouseCost: 25, HotelCost: 100, Amount: 25, Value: 25},
			{ID: "cwb-c5", Deck: CardTypeChance, Type: CardTypeChance, Title: "Radar na Linha Verde", Text: "Infração de trânsito em faixa exclusiva. Pague R$ 50 de multa.", Effect: CardActionMoney, Action: CardActionMoney, Amount: -50, Value: -50},
			{ID: "cwb-c6", Deck: CardTypeChance, Type: CardTypeChance, Title: "Festival de Teatro", Text: "Temporada de espetáculos com ingressos esgotados: receba R$ 50 de cada jogador.", Effect: CardActionCollectFromPlayers, Action: CardActionCollectFromPlayers, Amount: 50, Value: 50},
			{ID: "cwb-c7", Deck: CardTypeChance, Type: CardTypeChance, Title: "Vá para a Prisão", Text: "Irregularidades em licitação no Centro Cívico. Vá direto para a Prisão sem passar pelo GO.", Effect: CardActionGoToJail, Action: CardActionGoToJail},
		},
		Chest: []Card{
			{ID: "cwb-k1", Deck: CardTypeChest, Type: CardTypeChest, Title: "Calçadão da Rua XV", Text: "Receita recorde com quiosques de flores e café: receba R$ 200.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 200, Value: 200},
			{ID: "cwb-k2", Deck: CardTypeChest, Type: CardTypeChest, Title: "Taxa de Saneamento", Text: "Taxa extraordinária de conservação urbana de Curitiba. Pague R$ 100.", Effect: CardActionMoney, Action: CardActionMoney, Amount: -100, Value: -100},
			{ID: "cwb-k3", Deck: CardTypeChest, Type: CardTypeChest, Title: "Habeas Corpus", Text: "Saída Livre da Prisão! Guarde esta carta para quando precisar.", Effect: CardActionGetOutOfJail, Action: CardActionGetOutOfJail},
			{ID: "cwb-k4", Deck: CardTypeChest, Type: CardTypeChest, Title: "Largo da Ordem", Text: "Sucesso de vendas na feira de artesanato e gastronomia: receba R$ 100.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 100, Value: 100},
			{ID: "cwb-k5", Deck: CardTypeChest, Type: CardTypeChest, Title: "Defesa Civil", Text: "Vistoria preventiva em encostas de parques: pague R$ 50.", Effect: CardActionMoney, Action: CardActionMoney, Amount: -50, Value: -50},
			{ID: "cwb-k6", Deck: CardTypeChest, Type: CardTypeChest, Title: "Vá para a Prisão", Text: "Conduzido pelas autoridades municipais à carceragem.", Effect: CardActionGoToJail, Action: CardActionGoToJail},
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
		{ID: "gen-c1", Deck: CardTypeChance, Type: CardTypeChance, Title: "Especulação Imobiliária", Text: "Plano diretor da cidade aprovou novos índices. Receba R$ 150.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 150, Value: 150},
		{ID: "gen-c2", Deck: CardTypeChance, Type: CardTypeChance, Title: "Avançar até o GO", Text: "Avance imediatamente até a Saída e recolha R$ 200.", Effect: CardActionMovePos, Action: CardActionMovePos, TargetPos: 0},
		{ID: "gen-c3", Deck: CardTypeChance, Type: CardTypeChance, Title: "Multa de Obras", Text: "Falta de alvará provisório de construção. Pague R$ 50 ao banco.", Effect: CardActionMoney, Action: CardActionMoney, Amount: -50, Value: -50},
		{ID: "gen-c4", Deck: CardTypeChance, Type: CardTypeChance, Title: "Vá para a Prisão", Text: "Investigação fiscal detectou desvios. Vá direto para a Prisão.", Effect: CardActionGoToJail, Action: CardActionGoToJail},
		{ID: "gen-c5", Deck: CardTypeChance, Type: CardTypeChance, Title: "Reforma de Infraestrutura", Text: "Pague taxa de melhoria: R$ 25 por cada casa e R$ 100 por cada hotel.", Effect: CardActionRepairs, Action: CardActionRepairs, HouseCost: 25, HotelCost: 100, Amount: 25, Value: 25},
		{ID: "gen-c6", Deck: CardTypeChance, Type: CardTypeChance, Title: "Rateio dos Acionistas", Text: "Todos os concorrentes pagam dividendos a você: receba R$ 50 de cada um.", Effect: CardActionCollectFromPlayers, Action: CardActionCollectFromPlayers, Amount: 50, Value: 50},
	},
	Chest: []Card{
		{ID: "gen-k1", Deck: CardTypeChest, Type: CardTypeChest, Title: "Erro do Banco a seu Favor", Text: "Auditoria contábil estornou taxas indevidas: receba R$ 200.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 200, Value: 200},
		{ID: "gen-k2", Deck: CardTypeChest, Type: CardTypeChest, Title: "IPTU e Taxas Municipais", Text: "Carnê de impostos municipais vencido: pague R$ 100.", Effect: CardActionMoney, Action: CardActionMoney, Amount: -100, Value: -100},
		{ID: "gen-k3", Deck: CardTypeChest, Type: CardTypeChest, Title: "Habeas Corpus", Text: "Carta de Saída Livre da Prisão. Guarde-a ou use quando detido.", Effect: CardActionGetOutOfJail, Action: CardActionGetOutOfJail},
		{ID: "gen-k4", Deck: CardTypeChest, Type: CardTypeChest, Title: "Venda de Terreno Periférico", Text: "Loteamento comercial vendido com ágio: receba R$ 100.", Effect: CardActionMoney, Action: CardActionMoney, Amount: 100, Value: 100},
		{ID: "gen-k5", Deck: CardTypeChest, Type: CardTypeChest, Title: "Seguro Predial", Text: "Custo anual de proteção contra riscos civis: pague R$ 50.", Effect: CardActionMoney, Action: CardActionMoney, Amount: -50, Value: -50},
		{ID: "gen-k6", Deck: CardTypeChest, Type: CardTypeChest, Title: "Vá para a Prisão", Text: "Mandado de prisão temporária expedido.", Effect: CardActionGoToJail, Action: CardActionGoToJail},
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
		if newPos == 0 {
			p.Money += 400
			result.MoneyDelta += 400
		} else if newPos < oldPos {
			p.Money += 200
			result.MoneyDelta += 200
		}
		g.resolveTileLanding(p)
	case CardActionMoveRelative:
		p.Position = (p.Position + amount + len(g.Board.Tiles)) % len(g.Board.Tiles)
		result.NewPosition = p.Position
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
	}

	g.LastCard = &card
	return result, nil
}

// ApplyCard is an alias for ResolveCard compatibility.
func (g *Game) ApplyCard(playerID string, card Card) error {
	_, err := g.ResolveCard(playerID, card)
	return err
}
