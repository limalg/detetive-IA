package banco3d

import "fmt"

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
	Index     int        `json:"index"`
	Name      string     `json:"name"`
	Type      TileType   `json:"type"`
	Group     ColorGroup `json:"group"`
	Price     int        `json:"price"`      // 0 = não comprável
	Rent      [6]int     `json:"rent"`       // [base, 1c, 2c, 3c, 4c, hotel]
	HouseCost int        `json:"house_cost"`
	OwnerID   string     `json:"owner_id"`   // "" = banco
	Houses    int        `json:"houses"`     // 0..4, 5 = hotel
	IsQueen   bool       `json:"is_queen"`   // true somente no índice 39
	TaxAmount int        `json:"tax_amount,omitempty"`
}

// Board represents the 40-tile game board for a specific city.
type Board struct {
	City  string `json:"city"`
	Tiles []Tile `json:"tiles"`
}

// Default property rents and pricing per color group from Oracle specification.
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
		return 320, [6]int{26, 130, 390, 900, 1100, 1275}, 200
	case ColorDarkBlue:
		return 400, [6]int{35, 175, 500, 1100, 1300, 1500}, 200
	default:
		return 0, [6]int{}, 0
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
		if isQueen {
			price = 800
			rent = [6]int{60, 300, 900, 1750, 2000, 2500}
			houseCost = 200
		}
		return Tile{
			Index:     idx,
			Name:      name,
			Type:      TileProperty,
			Group:     group,
			Price:     price,
			Rent:      rent,
			HouseCost: houseCost,
			IsQueen:   isQueen,
		}
	}

	makeStation := func(idx int, name string) Tile {
		return Tile{
			Index: idx,
			Name:  name,
			Type:  TileStation,
			Group: ColorNone,
			Price: 200,
			Rent:  [6]int{25, 50, 100, 200, 0, 0},
		}
	}

	makeUtility := func(idx int, name string) Tile {
		return Tile{
			Index: idx,
			Name:  name,
			Type:  TileUtility,
			Group: ColorNone,
			Price: 150,
		}
	}

	// 40 squares layout (22 properties: 2 Brown, 3 LightBlue, 3 Pink, 3 Orange, 3 Red, 3 Yellow, 3 Green, 2 DarkBlue)
	tiles[0] = Tile{Index: 0, Name: "SAÍDA", Type: TileGo, Group: ColorNone}
	tiles[1] = makeProp(1, fmt.Sprintf("Marrom A (%s)", cityName), ColorBrown, false)
	tiles[2] = makeProp(2, fmt.Sprintf("Marrom B (%s)", cityName), ColorBrown, false)
	tiles[3] = makeStation(3, "Estação Central")
	tiles[4] = makeProp(4, fmt.Sprintf("Azul-claro A (%s)", cityName), ColorLightBlue, false)
	tiles[5] = Tile{Index: 5, Name: "Taxa Municipal", Type: TileTax, Group: ColorNone, TaxAmount: 100}
	tiles[6] = makeProp(6, fmt.Sprintf("Azul-claro B (%s)", cityName), ColorLightBlue, false)
	tiles[7] = makeProp(7, fmt.Sprintf("Azul-claro C (%s)", cityName), ColorLightBlue, false)
	tiles[8] = makeUtility(8, "Companhia de Energia")
	tiles[9] = makeProp(9, fmt.Sprintf("Rosa A (%s)", cityName), ColorPink, false)
	tiles[10] = Tile{Index: 10, Name: "PRISÃO", Type: TileJail, Group: ColorNone}
	tiles[11] = makeProp(11, fmt.Sprintf("Rosa B (%s)", cityName), ColorPink, false)
	tiles[12] = makeProp(12, fmt.Sprintf("Rosa C (%s)", cityName), ColorPink, false)
	tiles[13] = makeStation(13, "Terminal Norte")
	tiles[14] = makeProp(14, fmt.Sprintf("Laranja A (%s)", cityName), ColorOrange, false)
	tiles[15] = Tile{Index: 15, Name: "Sorte", Type: TileChance, Group: ColorNone}
	tiles[16] = makeProp(16, fmt.Sprintf("Laranja B (%s)", cityName), ColorOrange, false)
	tiles[17] = makeProp(17, fmt.Sprintf("Laranja C (%s)", cityName), ColorOrange, false)
	tiles[18] = Tile{Index: 18, Name: "Revés", Type: TileChest, Group: ColorNone}
	tiles[19] = Tile{Index: 19, Name: "Sorte", Type: TileChance, Group: ColorNone}
	tiles[20] = Tile{Index: 20, Name: "VISITANTE", Type: TileFreeParking, Group: ColorNone}
	tiles[21] = makeProp(21, fmt.Sprintf("Vermelho A (%s)", cityName), ColorRed, false)
	tiles[22] = makeProp(22, fmt.Sprintf("Vermelho B (%s)", cityName), ColorRed, false)
	tiles[23] = makeProp(23, fmt.Sprintf("Vermelho C (%s)", cityName), ColorRed, false)
	tiles[24] = makeProp(24, fmt.Sprintf("Amarelo A (%s)", cityName), ColorYellow, false)
	tiles[25] = makeStation(25, "Estação Leste")
	tiles[26] = makeProp(26, fmt.Sprintf("Amarelo B (%s)", cityName), ColorYellow, false)
	tiles[27] = makeProp(27, fmt.Sprintf("Amarelo C (%s)", cityName), ColorYellow, false)
	tiles[28] = makeUtility(28, "Companhia de Água")
	tiles[29] = makeProp(29, fmt.Sprintf("Verde A (%s)", cityName), ColorGreen, false)
	tiles[30] = Tile{Index: 30, Name: "VÁ PARA A CADEIA", Type: TileGoToJail, Group: ColorNone}
	tiles[31] = makeProp(31, fmt.Sprintf("Verde B (%s)", cityName), ColorGreen, false)
	tiles[32] = makeProp(32, fmt.Sprintf("Verde C (%s)", cityName), ColorGreen, false)
	tiles[33] = makeStation(33, "Aeroporto Internacional")
	tiles[34] = Tile{Index: 34, Name: "Sorte", Type: TileChance, Group: ColorNone}
	tiles[35] = Tile{Index: 35, Name: "Revés", Type: TileChest, Group: ColorNone}
	tiles[36] = makeProp(36, fmt.Sprintf("Azul-escuro Secundária (%s)", cityName), ColorDarkBlue, false)
	tiles[37] = Tile{Index: 37, Name: "Imposto de Luxo", Type: TileTax, Group: ColorNone, TaxAmount: 200}
	tiles[38] = Tile{Index: 38, Name: "Revés", Type: TileChest, Group: ColorNone}
	tiles[39] = makeProp(39, fmt.Sprintf("Ponto Mais Icônico (%s)", cityName), ColorDarkBlue, true) // Rainha

	return &Board{City: cityName, Tiles: tiles}
}
