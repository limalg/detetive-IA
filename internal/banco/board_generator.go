package banco

import (
	"fmt"
	"strings"
)

// GenerateBoard creates a balanced 28-tile board tailored to the specified city.
func GenerateBoard(city string, schoolGrade string, age int) []Tile {
	cleanCity := strings.TrimSpace(city)
	if cleanCity == "" {
		cleanCity = "São Paulo"
	}

	normCity := strings.ToLower(cleanCity)
	switch {
	case strings.Contains(normCity, "paulo"):
		return generateSaoPauloBoard(cleanCity)
	case strings.Contains(normCity, "rio") || strings.Contains(normCity, "janeiro"):
		return generateRioBoard(cleanCity)
	case strings.Contains(normCity, "curitiba"):
		return generateCuritibaBoard(cleanCity)
	default:
		return generateProceduralBoard(cleanCity)
	}
}

func generateSaoPauloBoard(city string) []Tile {
	return buildBoardFromNames(city, boardNames{
		brown1: "Praça da Sé",
		brown2: "Mooca",
		station1: "Estação da Luz",
		ltBlue1: "Consolação",
		ltBlue2: "Bela Vista",
		util1: "Companhia Enel SP",
		pink1: "Vila Madalena",
		pink2: "Pinheiros",
		station2: "Terminal Barra Funda",
		orange1: "Perdizes",
		orange2: "Higienópolis",
		util2: "Sabesp Saneamento",
		red1: "Parque Ibirapuera",
		station3: "Estação Brás",
		red2: "Jardins",
		green1: "Avenida Faria Lima",
		station4: "Aeroporto de Congonhas",
		darkBlue1: "Avenida Paulista",
	})
}

func generateRioBoard(city string) []Tile {
	return buildBoardFromNames(city, boardNames{
		brown1: "Lapa",
		brown2: "Santa Teresa",
		station1: "Central do Brasil",
		ltBlue1: "Botafogo",
		ltBlue2: "Flamengo",
		util1: "Light Energia Rio",
		pink1: "Tijuca",
		pink2: "Laranjeiras",
		station2: "Estação Maracanã",
		orange1: "Barra da Tijuca",
		orange2: "São Conrado",
		util2: "Cedae Águas do Rio",
		red1: "Copacabana",
		station3: "Terminal Alvorada",
		red2: "Ipanema",
		green1: "Lagoa Rodrigo de Freitas",
		station4: "Aeroporto Santos Dumont",
		darkBlue1: "Avenida Vieira Souto (Leblon)",
	})
}

func generateCuritibaBoard(city string) []Tile {
	return buildBoardFromNames(city, boardNames{
		brown1: "Largo da Ordem",
		brown2: "São Francisco",
		station1: "Rodoferroviária Central",
		ltBlue1: "Jardim Botânico",
		ltBlue2: "Cristo Rei",
		util1: "Copel Energia",
		pink1: "Mercês",
		pink2: "Bigorrilho (Champagnat)",
		station2: "Terminal Guadalupe",
		orange1: "Santa Felicidade",
		orange2: "Cabral",
		util2: "Sanepar Águas",
		red1: "Rua XV de Novembro",
		station3: "Estação Tubo Bento Viana",
		red2: "Centro Cívico",
		green1: "Batel Soho",
		station4: "Aeroporto Afonso Pena",
		darkBlue1: "Avenida do Batel",
	})
}

func generateProceduralBoard(city string) []Tile {
	return buildBoardFromNames(city, boardNames{
		brown1: fmt.Sprintf("Centro Histórico de %s", city),
		brown2: fmt.Sprintf("Bairro Operário de %s", city),
		station1: fmt.Sprintf("Terminal Ferroviário Central de %s", city),
		ltBlue1: fmt.Sprintf("Jardim Universitário de %s", city),
		ltBlue2: fmt.Sprintf("Bairro dos Comerciantes de %s", city),
		util1: fmt.Sprintf("Usina de Energia de %s", city),
		pink1: fmt.Sprintf("Vila Boêmia de %s", city),
		pink2: fmt.Sprintf("Bairro das Artes de %s", city),
		station2: fmt.Sprintf("Estação Norte de %s", city),
		orange1: fmt.Sprintf("Distrito Financeiro de %s", city),
		orange2: fmt.Sprintf("Avenida Central de %s", city),
		util2: fmt.Sprintf("Companhia de Águas de %s", city),
		red1: fmt.Sprintf("Parque das Águas de %s", city),
		station3: fmt.Sprintf("Terminal Oeste de %s", city),
		red2: fmt.Sprintf("Boulevard Real de %s", city),
		green1: fmt.Sprintf("Polo de Alta Tecnologia de %s", city),
		station4: fmt.Sprintf("Aeroporto Internacional de %s", city),
		darkBlue1: fmt.Sprintf("Mansões da Colina de %s", city),
	})
}

type boardNames struct {
	brown1, brown2       string
	station1             string
	ltBlue1, ltBlue2     string
	util1                string
	pink1, pink2         string
	station2             string
	orange1, orange2     string
	util2                string
	red1, red2           string
	station3             string
	green1               string
	station4             string
	darkBlue1            string
}

func buildBoardFromNames(city string, names boardNames) []Tile {
	return []Tile{
		// Corner 0: START
		{Position: 0, Type: TileStart, Name: "Ponto de Partida", Group: "special"},

		// Side 1 (1 to 6)
		{Position: 1, Type: TileProperty, Name: names.brown1, Group: "brown", Price: 60, RentBase: 2, RentHouse: [5]int{10, 30, 90, 160, 250}, BuildCost: 50},
		{Position: 2, Type: TileLuck, Name: "Sorte ou Revés", Group: "special"},
		{Position: 3, Type: TileProperty, Name: names.brown2, Group: "brown", Price: 80, RentBase: 4, RentHouse: [5]int{20, 60, 180, 320, 450}, BuildCost: 50},
		{Position: 4, Type: TileTax, Name: "Taxa Municipal", Group: "special", Price: 100},
		{Position: 5, Type: TileStation, Name: names.station1, Group: "station", Price: 200, RentBase: 25},
		{Position: 6, Type: TileProperty, Name: names.ltBlue1, Group: "light_blue", Price: 100, RentBase: 6, RentHouse: [5]int{30, 90, 270, 400, 550}, BuildCost: 50},

		// Corner 7: JAIL
		{Position: 7, Type: TileJail, Name: "Prisão (Visita)", Group: "special"},

		// Side 2 (8 to 13)
		{Position: 8, Type: TileProperty, Name: names.ltBlue2, Group: "light_blue", Price: 120, RentBase: 8, RentHouse: [5]int{40, 100, 300, 450, 600}, BuildCost: 50},
		{Position: 9, Type: TileUtility, Name: names.util1, Group: "utility", Price: 150, RentBase: 10},
		{Position: 10, Type: TileProperty, Name: names.pink1, Group: "pink", Price: 140, RentBase: 10, RentHouse: [5]int{50, 150, 450, 625, 750}, BuildCost: 100},
		{Position: 11, Type: TileLuck, Name: "Sorte ou Revés", Group: "special"},
		{Position: 12, Type: TileProperty, Name: names.pink2, Group: "pink", Price: 160, RentBase: 12, RentHouse: [5]int{60, 180, 500, 700, 900}, BuildCost: 100},
		{Position: 13, Type: TileStation, Name: names.station2, Group: "station", Price: 200, RentBase: 25},

		// Corner 14: PARKING
		{Position: 14, Type: TileParking, Name: "Parada Livre", Group: "special"},

		// Side 3 (15 to 20)
		{Position: 15, Type: TileProperty, Name: names.orange1, Group: "orange", Price: 180, RentBase: 14, RentHouse: [5]int{70, 200, 550, 750, 950}, BuildCost: 100},
		{Position: 16, Type: TileLuck, Name: "Sorte ou Revés", Group: "special"},
		{Position: 17, Type: TileProperty, Name: names.orange2, Group: "orange", Price: 200, RentBase: 16, RentHouse: [5]int{80, 220, 600, 800, 1000}, BuildCost: 100},
		{Position: 18, Type: TileUtility, Name: names.util2, Group: "utility", Price: 150, RentBase: 10},
		{Position: 19, Type: TileProperty, Name: names.red1, Group: "red", Price: 220, RentBase: 18, RentHouse: [5]int{90, 250, 700, 875, 1050}, BuildCost: 150},
		{Position: 20, Type: TileStation, Name: names.station3, Group: "station", Price: 200, RentBase: 25},

		// Corner 21: GO TO JAIL
		{Position: 21, Type: TileGoToJail, Name: "Vá para a Prisão!", Group: "special"},

		// Side 4 (22 to 27)
		{Position: 22, Type: TileProperty, Name: names.red2, Group: "red", Price: 240, RentBase: 20, RentHouse: [5]int{100, 300, 750, 925, 1100}, BuildCost: 150},
		{Position: 23, Type: TileLuck, Name: "Sorte ou Revés", Group: "special"},
		{Position: 24, Type: TileProperty, Name: names.green1, Group: "green", Price: 280, RentBase: 24, RentHouse: [5]int{120, 360, 850, 1025, 1200}, BuildCost: 150},
		{Position: 25, Type: TileTax, Name: "Taxa de Luxo", Group: "special", Price: 150},
		{Position: 26, Type: TileStation, Name: names.station4, Group: "station", Price: 200, RentBase: 25},
		{Position: 27, Type: TileProperty, Name: names.darkBlue1, Group: "dark_blue", Price: 350, RentBase: 35, RentHouse: [5]int{175, 500, 1100, 1300, 1500}, BuildCost: 200},
	}
}

// GenerateLuckCards builds thematic luck and setback cards contextualized for the city.
func GenerateLuckCards(city string) []Card {
	return []Card{
		{
			ID:          "card_1",
			Type:        "luck",
			Title:       "Dividendo Imobiliário",
			Description: fmt.Sprintf("Seus investimentos comerciais em %s valorizaram após a expansão do metrô! Receba R$ 150.", city),
			Amount:      150,
		},
		{
			ID:          "card_2",
			Type:        "setback",
			Title:       "Manutenção Predial",
			Description: fmt.Sprintf("Reformas estruturais urgentes foram exigidas pela prefeitura de %s. Pague R$ 100.", city),
			Amount:      -100,
		},
		{
			ID:          "card_3",
			Type:        "luck",
			Title:       "Prêmio de Empreendedorismo",
			Description: fmt.Sprintf("Você venceu o prêmio de inovação urbana da Associação Comercial de %s! Receba R$ 200.", city),
			Amount:      200,
		},
		{
			ID:          "card_4",
			Type:        "setback",
			Title:       "Multa de Trânsito & Licenças",
			Description: fmt.Sprintf("Atraso na renovação do alvará de funcionamento no centro de %s. Pague R$ 50.", city),
			Amount:      -50,
		},
		{
			ID:          "card_5",
			Type:        "luck",
			Title:       "Restituição de Impostos",
			Description: "A receita municipal recalculou as taxas prediais a seu favor. Receba R$ 80.",
			Amount:      80,
		},
		{
			ID:          "card_6",
			Type:        "setback",
			Title:       "Conta de Energia Elétrica",
			Description: "Pico de tarifa na temporada de calor e ar-condicionado. Pague R$ 60.",
			Amount:      -60,
		},
	}
}
