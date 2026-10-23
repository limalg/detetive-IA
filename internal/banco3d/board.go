package banco3d

import (
	"fmt"
	"strings"
	"time"
)

// TileType represents the type of a board tile.
type TileType string

const (
	TileGo          TileType = "GO"
	TileProperty    TileType = "PROPERTY"
	TileStation     TileType = "STATION"
	TileUtility     TileType = "UTILITY"
	TileTax         TileType = "TAX"
	TileChance      TileType = "CHANCE" // Sorte
	TileChest       TileType = "CHEST"  // Revés
	TileJail        TileType = "JAIL"
	TileFreeParking TileType = "FREE_PARKING"
	TileGoToJail    TileType = "GO_TO_JAIL"
)

// ColorGroup represents the color category of a property.
type ColorGroup string

const (
	ColorBrown     ColorGroup = "brown"
	ColorLightBlue ColorGroup = "light_blue"
	ColorPink      ColorGroup = "pink"
	ColorOrange    ColorGroup = "orange"
	ColorRed       ColorGroup = "red"
	ColorYellow    ColorGroup = "yellow"
	ColorGreen     ColorGroup = "green"
	ColorDarkBlue  ColorGroup = "dark_blue"
	ColorNone      ColorGroup = "none"
)

// Tile defines a single square on the 40-tile board.
type Tile struct {
	Index         int        `json:"index"`
	Name          string     `json:"name"`
	Type          TileType   `json:"type"`
	Group         ColorGroup `json:"group"`
	Price         int        `json:"price"` // 0 = não comprável
	Rent          [6]int     `json:"rent"`  // [base, 1c, 2c, 3c, 4c, hotel]
	HouseCost     int        `json:"house_cost"`
	OwnerID       string     `json:"owner_id"` // "" = banco
	Houses        int        `json:"houses"`   // 0..4, 5 = hotel
	IsQueen       bool       `json:"is_queen"` // true somente no índice 39
	TaxAmount     int        `json:"tax_amount,omitempty"`
	MortgageValue int        `json:"mortgage_value,omitempty"`
	IsMortgaged   bool       `json:"is_mortgaged,omitempty"`
	Mortgaged     bool       `json:"mortgaged,omitempty"`
	Description   string     `json:"description,omitempty"`
	ImageKind     string     `json:"image_kind,omitempty"` // street, avenue, square, park, landmark, station, utility
	DarkMode      bool       `json:"dark_mode,omitempty"`
	FestivalMultiplier     int        `json:"festival_multiplier,omitempty"`
	FestivalRounds         int        `json:"festival_rounds,omitempty"`
	FestivalUntil          *time.Time `json:"festival_until,omitempty"`
	ChampionshipMultiplier int        `json:"championship_multiplier,omitempty"`
	ChampionshipRounds     int        `json:"championship_rounds,omitempty"`
	ChampionshipUntil      *time.Time `json:"championship_until,omitempty"`
}

// Board represents the 40-tile game board for a specific city.
type Board struct {
	City  string `json:"city"`
	Tiles []Tile `json:"tiles"`
}

// GetGroupIndices returns all tile indices belonging to a given color group.
func (b *Board) GetGroupIndices(group ColorGroup) []int {
	if group == ColorNone {
		return nil
	}
	var res []int
	for i := range b.Tiles {
		if b.Tiles[i].Type == TileProperty && b.Tiles[i].Group == group {
			res = append(res, i)
		}
	}
	return res
}

// Default property rents and pricing per color group from Oracle / Monopoly / Business Tour specification.
func getGroupSpecs(group ColorGroup) (price int, rent [6]int, houseCost int) {
	switch group {
	case ColorBrown:
		return 60, [6]int{2, 10, 30, 90, 160, 250}, 50
	case ColorLightBlue:
		return 100, [6]int{6, 30, 90, 270, 400, 550}, 50
	case ColorPink:
		return 140, [6]int{10, 50, 150, 450, 625, 750}, 100
	case ColorOrange:
		return 180, [6]int{14, 70, 200, 550, 750, 950}, 100
	case ColorRed:
		return 220, [6]int{18, 90, 250, 700, 875, 1050}, 150
	case ColorYellow:
		return 260, [6]int{20, 100, 300, 750, 925, 1100}, 150
	case ColorGreen:
		return 300, [6]int{26, 130, 390, 900, 1100, 1275}, 200
	case ColorDarkBlue:
		return 350, [6]int{35, 175, 500, 1100, 1300, 1500}, 200
	default:
		return 0, [6]int{}, 0
	}
}

// ClassifyImageKind infers the appropriate image category for a given tile.
func ClassifyImageKind(name string, t TileType) string {
	switch t {
	case TileStation:
		return "station"
	case TileUtility:
		return "utility"
	case TileProperty:
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "rua") || strings.Contains(lower, "calçadão") || strings.Contains(lower, "beco") || strings.Contains(lower, "travessa") {
			return "street"
		}
		if strings.HasPrefix(lower, "avenida") || strings.HasPrefix(lower, "av.") || strings.HasPrefix(lower, "alameda") || strings.HasPrefix(lower, "rodovia") {
			return "avenue"
		}
		if strings.HasPrefix(lower, "praça") || strings.HasPrefix(lower, "praca") || strings.HasPrefix(lower, "largo") {
			return "square"
		}
		if strings.HasPrefix(lower, "parque") || strings.HasPrefix(lower, "jardim") || strings.HasPrefix(lower, "bosque") {
			return "park"
		}
		return "landmark"
	default:
		return ""
	}
}

// NewStandardBoard builds a 40-tile board for the given city.
func NewStandardBoard(cityName string, customTiles []Tile) *Board {
	if len(customTiles) == 40 {
		return &Board{City: cityName, Tiles: customTiles}
	}

	tiles := make([]Tile, 40)

	makeProp := func(idx int, name string, group ColorGroup, isQueen bool) Tile {
		price, rent, houseCost := getGroupSpecs(group)
		// Specific price & rent overrides for ascending progression within groups matching Business Tour & Monopoly:
		switch idx {
		case 3:
			price = 60
			rent = [6]int{4, 20, 60, 180, 320, 450}
		case 9:
			price = 120
			rent = [6]int{8, 40, 100, 300, 450, 600}
		case 14:
			price = 160
			rent = [6]int{12, 60, 180, 500, 700, 900}
		case 19:
			price = 200
			rent = [6]int{16, 80, 220, 600, 800, 1000}
		case 24:
			price = 240
			rent = [6]int{20, 100, 300, 750, 925, 1100}
		case 29:
			price = 280
			rent = [6]int{24, 120, 360, 850, 1025, 1200}
		case 34:
			price = 320
			rent = [6]int{28, 150, 450, 1000, 1200, 1400}
		case 37:
			price = 350
			rent = [6]int{35, 175, 500, 1100, 1300, 1500}
		case 39:
			price = 400
			rent = [6]int{50, 200, 600, 1400, 1700, 2000}
		}
		if isQueen {
			price = 400
			rent = [6]int{50, 200, 600, 1400, 1700, 2000}
			houseCost = 200
		}
		imgKind := ClassifyImageKind(name, TileProperty)
		if isQueen {
			imgKind = "landmark"
		}
		return Tile{
			Index:         idx,
			Name:          name,
			Type:          TileProperty,
			Group:         group,
			Price:         price,
			Rent:          rent,
			HouseCost:     houseCost,
			IsQueen:       isQueen,
			MortgageValue: price / 2,
			ImageKind:     imgKind,
			Description:   fmt.Sprintf("Propriedade clássica do grupo %s em %s", group, cityName),
		}
	}

	makeStation := func(idx int, name string) Tile {
		return Tile{
			Index:         idx,
			Name:          name,
			Type:          TileStation,
			Group:         ColorNone,
			Price:         200,
			Rent:          [6]int{25, 50, 100, 200, 0, 0},
			MortgageValue: 100,
			ImageKind:     "station_train",
			Description:   "Terminal de transporte metropolitano",
		}
	}

	makeUtility := func(idx int, name string, bolt bool) Tile {
		img := "utility_bolt"
		if !bolt {
			img = "utility_drop"
		}
		return Tile{
			Index:         idx,
			Name:          name,
			Type:          TileUtility,
			Group:         ColorNone,
			Price:         150,
			MortgageValue: 75,
			ImageKind:     img,
			Description:   "Concessionária pública essencial de serviços",
		}
	}

	// 40 squares layout (Canonical Business Tour / Monopoly layout: 22 properties, 4 stations, 2 utilities, 2 taxes, 3 chance, 3 chest, 4 corners)
	// Side 1 (0-9)
	tiles[0] = Tile{Index: 0, Name: "SAÍDA", Type: TileGo, Group: ColorNone, ImageKind: "go_flag", Description: "Ponto de partida clássico"}
	tiles[1] = makeProp(1, fmt.Sprintf("Marrom A (%s)", cityName), ColorBrown, false)
	tiles[2] = Tile{Index: 2, Name: "Revés", Type: TileChest, Group: ColorNone, ImageKind: "chance_card_chest", Description: "Carta de revés econômico"}
	tiles[3] = makeProp(3, fmt.Sprintf("Marrom B (%s)", cityName), ColorBrown, false)
	tiles[4] = Tile{Index: 4, Name: "Taxa Municipal", Type: TileTax, Group: ColorNone, TaxAmount: 200, ImageKind: "tax_receipt", Description: "Taxa municipal fixa de $ 200"}
	tiles[5] = makeStation(5, "Estação Sul")
	tiles[6] = makeProp(6, fmt.Sprintf("Azul-claro A (%s)", cityName), ColorLightBlue, false)
	tiles[7] = Tile{Index: 7, Name: "Sorte", Type: TileChance, Group: ColorNone, ImageKind: "chance_card_chest", Description: "Carta surpresa de sorte"}
	tiles[8] = makeProp(8, fmt.Sprintf("Azul-claro B (%s)", cityName), ColorLightBlue, false)
	tiles[9] = makeProp(9, fmt.Sprintf("Azul-claro C (%s)", cityName), ColorLightBlue, false)

	// Side 2 (10-19)
	tiles[10] = Tile{Index: 10, Name: "PRISÃO", Type: TileJail, Group: ColorNone, ImageKind: "jail_bars", Description: "Visitas ou cumprimento de pena"}
	tiles[11] = makeProp(11, fmt.Sprintf("Rosa A (%s)", cityName), ColorPink, false)
	tiles[12] = makeUtility(12, "Companhia de Energia", true)
	tiles[13] = makeProp(13, fmt.Sprintf("Rosa B (%s)", cityName), ColorPink, false)
	tiles[14] = makeProp(14, fmt.Sprintf("Rosa C (%s)", cityName), ColorPink, false)
	tiles[15] = makeStation(15, "Estação Oeste")
	tiles[16] = makeProp(16, fmt.Sprintf("Laranja A (%s)", cityName), ColorOrange, false)
	tiles[17] = Tile{Index: 17, Name: "Revés", Type: TileChest, Group: ColorNone, ImageKind: "chance_card_chest", Description: "Carta de revés econômico"}
	tiles[18] = makeProp(18, fmt.Sprintf("Laranja B (%s)", cityName), ColorOrange, false)
	tiles[19] = makeProp(19, fmt.Sprintf("Laranja C (%s)", cityName), ColorOrange, false)

	// Side 3 (20-29)
	tiles[20] = Tile{Index: 20, Name: "Campeonato", Type: TileFreeParking, Group: ColorNone, ImageKind: "trophy", Description: "Campeonato Mundial: aluguel ×3 na sua propriedade por 1 rodada"}
	tiles[21] = makeProp(21, fmt.Sprintf("Vermelho A (%s)", cityName), ColorRed, false)
	tiles[22] = Tile{Index: 22, Name: "Sorte", Type: TileChance, Group: ColorNone, ImageKind: "chance_card_chest", Description: "Carta surpresa de sorte"}
	tiles[23] = makeProp(23, fmt.Sprintf("Vermelho B (%s)", cityName), ColorRed, false)
	tiles[24] = makeProp(24, fmt.Sprintf("Vermelho C (%s)", cityName), ColorRed, false)
	tiles[25] = makeStation(25, "Estação Norte")
	tiles[26] = makeProp(26, fmt.Sprintf("Amarelo A (%s)", cityName), ColorYellow, false)
	tiles[27] = makeProp(27, fmt.Sprintf("Amarelo B (%s)", cityName), ColorYellow, false)
	tiles[28] = makeUtility(28, "Companhia de Água", false)
	tiles[29] = makeProp(29, fmt.Sprintf("Amarelo C (%s)", cityName), ColorYellow, false)

	// Side 4 (30-39)
	tiles[30] = Tile{Index: 30, Name: "VÁ PARA A CADEIA", Type: TileGoToJail, Group: ColorNone, ImageKind: "go_to_jail_handcuffs", Description: "Vá direto para a cadeia"}
	tiles[31] = makeProp(31, fmt.Sprintf("Verde A (%s)", cityName), ColorGreen, false)
	tiles[32] = makeProp(32, fmt.Sprintf("Verde B (%s)", cityName), ColorGreen, false)
	tiles[33] = Tile{Index: 33, Name: "Revés", Type: TileChest, Group: ColorNone, ImageKind: "chance_card_chest", Description: "Carta de revés econômico"}
	tiles[34] = makeProp(34, fmt.Sprintf("Verde C (%s)", cityName), ColorGreen, false)
	tiles[35] = makeStation(35, "Estação Leste")
	tiles[36] = Tile{Index: 36, Name: "Sorte", Type: TileChance, Group: ColorNone, ImageKind: "chance_card_chest", Description: "Carta surpresa de sorte"}
	tiles[37] = makeProp(37, fmt.Sprintf("Azul-escuro Secundária (%s)", cityName), ColorDarkBlue, false)
	tiles[38] = Tile{Index: 38, Name: "Imposto de Luxo", Type: TileTax, Group: ColorNone, TaxAmount: 100, ImageKind: "luxury_diamond", DarkMode: true, Description: "Tributo de luxo de $ 100"}
	queenTile := makeProp(39, fmt.Sprintf("Ponto Mais Icônico (%s)", cityName), ColorDarkBlue, true)
	queenTile.ImageKind = "prop_strip_queen"
	queenTile.DarkMode = true
	tiles[39] = queenTile

	return &Board{City: cityName, Tiles: tiles}
}
