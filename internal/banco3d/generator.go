package banco3d

import (
	"fmt"
	"strings"
)

type cityPreset struct {
	props      [22]string
	stations   [4]string
	utilities  [2]string
	candidates [4]string
}

// resolveCityPreset matches a query to a dedicated catalog preset or generates a procedural fallback.
func resolveCityPreset(query string) cityPreset {
	norm := strings.ToLower(strings.TrimSpace(query))
	switch {
	case strings.Contains(norm, "curitiba"):
		return curitibaPreset()
	case strings.Contains(norm, "londrina"):
		return londrinaPreset()
	case strings.Contains(norm, "recife"):
		return recifePreset()
	case strings.Contains(norm, "porto alegre") || strings.Contains(norm, "poa"):
		return portoAlegrePreset()
	case strings.Contains(norm, "brasília") || strings.Contains(norm, "brasilia") || strings.Contains(norm, "distrito federal"):
		return brasiliaPreset()
	case strings.Contains(norm, "fortaleza"):
		return fortalezaPreset()
	case strings.Contains(norm, "florianópolis") || strings.Contains(norm, "florianopolis") || strings.Contains(norm, "floripa"):
		return florianopolisPreset()
	case strings.Contains(norm, "são paulo") || strings.Contains(norm, "sao paulo") || strings.Contains(norm, "sp"):
		return saoPauloPreset()
	case strings.Contains(norm, "rio"):
		return rioPreset()
	case strings.Contains(norm, "belo horizonte") || strings.Contains(norm, "bh"):
		return beloHorizontePreset()
	case strings.Contains(norm, "salvador"):
		return salvadorPreset()
	case strings.Contains(norm, "nova york") || strings.Contains(norm, "new york") || strings.Contains(norm, "nyc"):
		return newYorkPreset()
	case strings.Contains(norm, "paris"):
		return parisPreset()
	default:
		return proceduralPreset(query)
	}
}

// GenerateCityBoard builds a 40-tile board and extracts 4 iconic candidates.
func GenerateCityBoard(query string, apiKey string) (*Board, []string, error) {
	clean := strings.TrimSpace(query)
	if clean == "" {
		clean = "Curitiba, PR"
	}

	preset := resolveCityPreset(clean)
	board := buildBoardFromPreset(clean, preset)
	return board, preset.candidates[:], nil
}

func buildBoardFromPreset(cityName string, p cityPreset) *Board {
	b := NewStandardBoard(cityName, nil)

	// Injetar nomes de propriedades (21 normais + 39 rainha/mais prestigiada)
	propIndices := []int{1, 3, 6, 8, 9, 11, 13, 14, 16, 18, 19, 21, 23, 24, 26, 27, 29, 31, 32, 34, 37}
	for i, idx := range propIndices {
		if i < len(p.props) {
			b.Tiles[idx].Name = p.props[i]
			b.Tiles[idx].ImageKind = ClassifyImageKind(p.props[i], TileProperty)
			b.Tiles[idx].MortgageValue = b.Tiles[idx].Price / 2
			b.Tiles[idx].Description = fmt.Sprintf("Local icônico em %s (%s)", cityName, p.props[i])
		}
	}

	// Casa rainha (índice 39) recebe inicialmente o primeiro candidato (será sobrescrito na votação)
	if len(p.candidates) > 0 && p.candidates[0] != "" {
		b.Tiles[39].Name = p.candidates[0]
	} else if len(p.props) > 21 {
		b.Tiles[39].Name = p.props[21]
	}
	b.Tiles[39].ImageKind = "landmark"
	b.Tiles[39].Price = 400
	b.Tiles[39].Rent = [6]int{50, 200, 600, 1400, 1700, 2000}
	b.Tiles[39].HouseCost = 200
	b.Tiles[39].MortgageValue = 200
	b.Tiles[39].Description = fmt.Sprintf("Propriedade Real mais prestigiada de %s", cityName)

	// Estações (4 posições clássicas: 5, 15, 25, 35)
	stIndices := []int{5, 15, 25, 35}
	for i, idx := range stIndices {
		if i < len(p.stations) {
			b.Tiles[idx].Name = p.stations[i]
			b.Tiles[idx].ImageKind = "station"
			b.Tiles[idx].MortgageValue = 100
		}
	}

	// Utilitários (2 posições clássicas: 12 e 28)
	if len(p.utilities) >= 2 {
		b.Tiles[12].Name = p.utilities[0]
		b.Tiles[12].ImageKind = "utility"
		b.Tiles[12].MortgageValue = 75
		b.Tiles[28].Name = p.utilities[1]
		b.Tiles[28].ImageKind = "utility"
		b.Tiles[28].MortgageValue = 75
	}

	return b
}

func curitibaPreset() cityPreset {
	return cityPreset{
		props: [22]string{
			"Largo da Ordem", "São Francisco",
			"Cristo Rei", "Jardim Social", "Alto da Glória",
			"Mercês", "Bigorrilho (Champagnat)", "Ahú",
			"Santa Felicidade", "Cabral", "Juvevê",
			"Batel Soho", "Água Verde", "Vila Izabel",
			"Centro Cívico", "Prado Velho", "Portão",
			"Ecoville (Mossunguê)", "Ecoville Park", "Alphaville Graciosa",
			"Avenida do Batel", "Parque Barigui",
		},
		stations: [4]string{
			"Rodoferroviária Central", "Terminal Guadalupe", "Terminal Campina do Siqueira", "Aeroporto Afonso Pena",
		},
		utilities: [2]string{"Copel Energia", "Sanepar Saneamento"},
		candidates: [4]string{
			"Jardim Botânico", "Ópera de Arame", "Parque Barigui", "Largo da Ordem",
		},
	}
}

func londrinaPreset() cityPreset {
	return cityPreset{
		props: [22]string{
			"Vila Nova", "Vila Brasil",
			"Higienópolis (Londrina)", "Jardim Bela Vista", "Jardim Quebec",
			"Gleba Palhano", "Avenida Ayrton Senna", "Alto da Palhano",
			"Centro Histórico", "Calçadão de Londrina", "Avenida Paraná",
			"Avenida Juscelino Kubitschek", "Jardim Maringá", "Parque Guanabara",
			"Parque Ney Braga", "Avenida Tiradentes", "Jardim Shangri-lá",
			"Avenida Higienópolis", "Praça Tomi Nakagawa", "Bosque Central",
			"Avenida Madre Leônia Milito", "Lago Igapó",
		},
		stations: [4]string{
			"Terminal Central de Londrina", "Terminal Rodoviário de Londrina", "Terminal Ouro Verde", "Aeroporto de Londrina",
		},
		utilities: [2]string{"Copel Londrina", "Sanepar Londrina"},
		candidates: [4]string{
			"Lago Igapó", "Calçadão de Londrina", "Catedral de Londrina", "Jardim Botânico de Londrina",
		},
	}
}

func saoPauloPreset() cityPreset {
	return cityPreset{
		props: [22]string{
			"Praça da Sé", "Mooca",
			"Consolação", "Bela Vista", "Liberdade",
			"Vila Madalena", "Pinheiros", "Perdizes",
			"Higienópolis", "Pompeia", "Aclimação",
			"Parque Ibirapuera", "Jardins", "Vila Nova Conceição",
			"Brooklin", "Campo Belo", "Chácara Santo Antônio",
			"Itaim Bibi", "Vila Olímpia", "Cidade Jardim",
			"Avenida Faria Lima", "Avenida Paulista",
		},
		stations: [4]string{
			"Estação da Luz", "Terminal Barra Funda", "Estação Brás", "Aeroporto de Congonhas",
		},
		utilities: [2]string{"Enel Distribuição SP", "Sabesp Águas"},
		candidates: [4]string{
			"Avenida Paulista", "Parque Ibirapuera", "Catedral da Sé", "Masp",
		},
	}
}

func rioPreset() cityPreset {
	return cityPreset{
		props: [22]string{
			"Lapa", "Santa Teresa",
			"Botafogo", "Flamengo", "Catete",
			"Tijuca", "Laranjeiras", "Grajaú",
			"Barra da Tijuca", "São Conrado", "Recreio dos Bandeirantes",
			"Copacabana", "Arpoador", "Urca",
			"Gávea", "Jardim Botânico (RJ)", "Humaitá",
			"Ipanema", "Lagoa Rodrigo de Freitas", "Joá",
			"Avenida Delfim Moreira", "Avenida Vieira Souto (Leblon)",
		},
		stations: [4]string{
			"Central do Brasil", "Estação Maracanã", "Terminal Alvorada", "Aeroporto Santos Dumont",
		},
		utilities: [2]string{"Light Energia Rio", "Cedae Águas"},
		candidates: [4]string{
			"Cristo Redentor", "Pão de Açúcar", "Copacabana", "Arcos da Lapa",
		},
	}
}

func beloHorizontePreset() cityPreset {
	return cityPreset{
		props: [22]string{
			"Lagoinha", "Bonfim",
			"Santa Tereza", "Floresta", "Prado",
			"Savassi", "Funcionários", "Lourdes",
			"Santo Agostinho", "Anchieta", "Cruzeiro",
			"Serra", "Mangabeiras", "Sion",
			"Buritis", "Gutierrez", "Coração de Jesus",
			"Belvedere", "Vila da Serra", "São Bento",
			"Praça da Liberdade", "Mirante das Mangabeiras",
		},
		stations: [4]string{
			"Estação Central BH", "Terminal Rodoviário", "Estação São Gabriel", "Aeroporto de Confins",
		},
		utilities: [2]string{"Cemig Energia", "Copasa MG"},
		candidates: [4]string{
			"Igreja da Pampulha", "Praça da Liberdade", "Mirante das Mangabeiras", "Mercado Central",
		},
	}
}

func salvadorPreset() cityPreset {
	return cityPreset{
		props: [22]string{
			"Comércio", "Calçada",
			"Santo Antônio Além do Carmo", "Nazaré", "Barbalho",
			"Rio Vermelho", "Pituba", "Brotas",
			"Graça", "Canela", "Federação",
			"Campo Grande", "Vitória (Corredor da Vitória)", "Barra",
			"Ondina", "Stella Maris", "Itapuã",
			"Caminho das Árvores", "Itaigara", "Horto Florestal",
			"Pelourinho", "Farol da Barra",
		},
		stations: [4]string{
			"Estação da Lapa", "Terminal Rodoviário de Salvador", "Estação Pirajá", "Aeroporto Internacional de Salvador",
		},
		utilities: [2]string{"Neoenergia Coelba", "Embasa Saneamento"},
		candidates: [4]string{
			"Elevador Lacerda", "Farol da Barra", "Pelourinho", "Igreja do Bonfim",
		},
	}
}

func recifePreset() cityPreset {
	return cityPreset{
		props: [22]string{
			"São José", "Santo Antônio",
			"Derby", "Graças", "Espinheiro",
			"Casa Forte", "Poço da Panela", "Santana",
			"Madalena", "Torre", "Prado",
			"Jaqueira", "Aflitos", "Tamarineira",
			"Pina", "Ilha do Leite", "Ilha do Retiro",
			"Boa Viagem", "Setúbal", "Piedade",
			"Avenida Boa Viagem", "Rua do Bom Jesus",
		},
		stations: [4]string{
			"Estação Central do Recife", "Terminal Integrado Joana Bezerra", "Terminal Integrado TIP", "Aeroporto dos Guararapes",
		},
		utilities: [2]string{"Neoenergia Pernambuco", "Compesa Saneamento"},
		candidates: [4]string{
			"Marco Zero", "Rua do Bom Jesus", "Praia de Boa Viagem", "Instituto Ricardo Brennand",
		},
	}
}

func portoAlegrePreset() cityPreset {
	return cityPreset{
		props: [22]string{
			"Cidade Baixa", "Bom Fim",
			"Menino Deus", "Praia de Belas", "Azenha",
			"Rio Branco", "Mont'Serrat", "Petrópolis",
			"Moinhos de Vento", "Independência", "Floresta",
			"Bela Vista (POA)", "Auxiliadora", "Higienópolis (POA)",
			"Três Figueiras", "Chácara das Pedras", "Jardim Botânico (POA)",
			"Ipanema (POA)", "Vila Assunção", "Tristeza",
			"Avenida Padre Chagas", "Orla do Guaíba",
		},
		stations: [4]string{
			"Estação Rodoviária de POA", "Estação Mercado (Trensurb)", "Terminal Triângulo", "Aeroporto Salgado Filho",
		},
		utilities: [2]string{"CEEE Equatorial", "DMAE Porto Alegre"},
		candidates: [4]string{
			"Orla do Guaíba", "Parque da Redenção", "Mercado Público de POA", "Usina do Gasômetro",
		},
	}
}

func brasiliaPreset() cityPreset {
	return cityPreset{
		props: [22]string{
			"Vila Planalto", "Candangolândia",
			"Asa Norte", "Asa Sul", "Sudoeste",
			"Noroeste", "Octogonal", "Cruzeiro",
			"Lago Norte", "Park Way", "Águas Claras",
			"Guará", "Taguatinga", "Vicente Pires",
			"Setor Hoteleiro Norte", "Setor Hoteleiro Sul", "Setor de Autarquias",
			"Lago Sul", "Península dos Ministros", "Pontão do Lago Sul",
			"Eixo Monumental", "Esplanada dos Ministérios",
		},
		stations: [4]string{
			"Rodoviária do Plano Piloto", "Terminal Interestadual de Brasília", "Estação Central (Metrô DF)", "Aeroporto Internacional de Brasília (JK)",
		},
		utilities: [2]string{"Neoenergia Brasília", "Caesb Saneamento"},
		candidates: [4]string{
			"Congresso Nacional", "Catedral Metropolitana de Brasília", "Ponte JK", "Praça dos Três Poderes",
		},
	}
}

func fortalezaPreset() cityPreset {
	return cityPreset{
		props: [22]string{
			"Praia de Iracema", "Centro Histórico de Fortaleza",
			"Benfica", "Fátima", "Montese",
			"Aldeota", "Meireles", "Varjota",
			"Dionísio Torres", "Joaquim Távora", "Papicu",
			"Cocó", "Guararapes", "Engenheiro Luciano Cavalcante",
			"Parquelândia", "Praça Portugal", "Água Fria",
			"Praia do Futuro", "Porto das Dunas", "Dunas",
			"Avenida Abolição", "Avenida Beira-Mar",
		},
		stations: [4]string{
			"Terminal Rodoviário Engenheiro João Thomé", "Terminal Integrado Papicu", "Terminal Parangaba", "Aeroporto Pinto Martins",
		},
		utilities: [2]string{"Enel Ceará", "Cagece Saneamento"},
		candidates: [4]string{
			"Praia do Futuro", "Dragão do Mar", "Beira-Mar de Fortaleza", "Catedral de Fortaleza",
		},
	}
}

func florianopolisPreset() cityPreset {
	return cityPreset{
		props: [22]string{
			"Centro de Florianópolis", "Prainha",
			"Trindade", "Itacorubi", "Córrego Grande",
			"Santa Mônica", "Pantanal", "Agronômica",
			"Lagoa da Conceição", "Barra da Lagoa", "Campeche",
			"Cacupé", "Sambaqui", "Santo Antônio de Lisboa",
			"Canasvieiras", "Ingleses", "Cachoeira do Bom Jesus",
			"Jurerê Tradicional", "Jurerê Internacional", "Praia Brava",
			"Avenida Beira-Mar Norte", "Ponte Hercílio Luz",
		},
		stations: [4]string{
			"Terminal Rodoviário Rita Maria", "Terminal Integração Centro (TICEN)", "Terminal Lagoa (TILAG)", "Aeroporto Hercílio Luz",
		},
		utilities: [2]string{"Celesc Distribuição", "Casan Saneamento"},
		candidates: [4]string{
			"Ponte Hercílio Luz", "Lagoa da Conceição", "Praia da Joaquina", "Santo Antônio de Lisboa",
		},
	}
}

func newYorkPreset() cityPreset {
	return cityPreset{
		props: [22]string{
			"Bowery", "Lower East Side",
			"Chinatown", "Little Italy", "Greenwich Village",
			"SoHo", "Tribeca", "Chelsea",
			"Hell's Kitchen", "Flatiron", "Gramercy",
			"Upper West Side", "Upper East Side", "Hudson Yards",
			"Midtown West", "Times Square", "East Village",
			"Financial District", "Battery Park", "DUMBO (Brooklyn)",
			"Park Avenue", "5th Avenue",
		},
		stations: [4]string{
			"Grand Central Terminal", "Penn Station", "World Trade Center Transit", "JFK Airport Station",
		},
		utilities: [2]string{"Con Edison", "NYC Environmental Protection"},
		candidates: [4]string{
			"Times Square", "Central Park", "Empire State Building", "Statue of Liberty",
		},
	}
}

func parisPreset() cityPreset {
	return cityPreset{
		props: [22]string{
			"Belleville", "Ménilmontant",
			"Montmartre", "Pigalle", "Bastille",
			"Le Marais", "Saint-Germain-des-Prés", "Quartier Latin",
			"Opéra", "Madeleine", "Montparnasse",
			"Invalides", "Trocadéro", "Passy",
			"Île Saint-Louis", "Île de la Cité", "Louvre-Rivoli",
			"Vendôme", "Champs-Élysées", "Faubourg Saint-Honoré",
			"Avenue Montaigne", "Torre Eiffel",
		},
		stations: [4]string{
			"Gare du Nord", "Gare de Lyon", "Gare Montparnasse", "Aéroport Charles de Gaulle",
		},
		utilities: [2]string{"EDF France", "Eau de Paris"},
		candidates: [4]string{
			"Torre Eiffel", "Museu do Louvre", "Arco do Triunfo", "Catedral de Notre-Dame",
		},
	}
}

func proceduralPreset(city string) cityPreset {
	clean := strings.TrimSpace(city)
	if clean == "" {
		clean = "Metrópole"
	}
	return cityPreset{
		props: [22]string{
			fmt.Sprintf("Centro Histórico de %s", clean), fmt.Sprintf("Bairro Operário de %s", clean),
			fmt.Sprintf("Vila Nova de %s", clean), fmt.Sprintf("Jardim América de %s", clean), fmt.Sprintf("Bairro São José de %s", clean),
			fmt.Sprintf("Parque das Flores de %s", clean), fmt.Sprintf("Bairro Universitário de %s", clean), fmt.Sprintf("Bairro Industrial de %s", clean),
			fmt.Sprintf("Bela Vista de %s", clean), fmt.Sprintf("Alto da Colina de %s", clean), fmt.Sprintf("Bairro dos Ipês de %s", clean),
			fmt.Sprintf("Avenida Central de %s", clean), fmt.Sprintf("Parque Ecológico de %s", clean), fmt.Sprintf("Mirante Verde de %s", clean),
			fmt.Sprintf("Bairro Imperial de %s", clean), fmt.Sprintf("Jardim Botânico de %s", clean), fmt.Sprintf("Vila Mariana de %s", clean),
			fmt.Sprintf("Condomínio dos Lagos de %s", clean), fmt.Sprintf("Alphaville de %s", clean), fmt.Sprintf("Residencial das Palmeiras de %s", clean),
			fmt.Sprintf("Avenida das Nações de %s", clean), fmt.Sprintf("Avenida Beira-Rio de %s", clean),
		},
		stations: [4]string{
			fmt.Sprintf("Terminal Rodoviário de %s", clean),
			fmt.Sprintf("Estação Ferroviária de %s", clean),
			fmt.Sprintf("Terminal Metropolitano de %s", clean),
			fmt.Sprintf("Aeroporto Regional de %s", clean),
		},
		utilities: [2]string{
			fmt.Sprintf("Companhia de Energia de %s", clean),
			fmt.Sprintf("Companhia de Água e Esgoto de %s", clean),
		},
		candidates: [4]string{
			fmt.Sprintf("Praça Central de %s", clean),
			fmt.Sprintf("Centro Histórico de %s", clean),
			fmt.Sprintf("Parque Ecológico de %s", clean),
			fmt.Sprintf("Mirante da Cidade de %s", clean),
		},
	}
}
