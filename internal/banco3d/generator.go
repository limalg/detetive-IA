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

// GenerateCityBoard builds a 40-tile board and extracts 4 iconic candidates.
func GenerateCityBoard(query string, apiKey string) (*Board, []string, error) {
	clean := strings.TrimSpace(query)
	if clean == "" {
		clean = "Curitiba, PR"
	}

	norm := strings.ToLower(clean)
	var preset cityPreset

	switch {
	case strings.Contains(norm, "curitiba"):
		preset = curitibaPreset()
	case strings.Contains(norm, "são paulo") || strings.Contains(norm, "sao paulo") || strings.Contains(norm, "sp"):
		preset = saoPauloPreset()
	case strings.Contains(norm, "rio"):
		preset = rioPreset()
	case strings.Contains(norm, "belo horizonte") || strings.Contains(norm, "bh"):
		preset = beloHorizontePreset()
	case strings.Contains(norm, "salvador"):
		preset = salvadorPreset()
	case strings.Contains(norm, "nova york") || strings.Contains(norm, "new york") || strings.Contains(norm, "nyc"):
		preset = newYorkPreset()
	case strings.Contains(norm, "paris"):
		preset = parisPreset()
	default:
		preset = proceduralPreset(clean)
	}

	board := buildBoardFromPreset(clean, preset)
	return board, preset.candidates[:], nil
}

func buildBoardFromPreset(cityName string, p cityPreset) *Board {
	b := NewStandardBoard(cityName, nil)

	// Injetar nomes de propriedades
	propIndices := []int{1, 2, 4, 6, 7, 9, 11, 12, 14, 16, 17, 19, 21, 22, 23, 24, 26, 27, 29, 31, 32, 36}
	for i, idx := range propIndices {
		if i < len(p.props) {
			b.Tiles[idx].Name = p.props[i]
		}
	}

	// Casa rainha (índice 39) recebe inicialmente o primeiro candidato (será sobrescrito na votação)
	b.Tiles[39].Name = p.candidates[0]

	// Estações
	stIndices := []int{3, 13, 25, 33}
	for i, idx := range stIndices {
		if i < len(p.stations) {
			b.Tiles[idx].Name = p.stations[i]
		}
	}

	// Utilitários
	if len(p.utilities) >= 2 {
		b.Tiles[8].Name = p.utilities[0]
		b.Tiles[28].Name = p.utilities[1]
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
	return cityPreset{
		props: [22]string{
			fmt.Sprintf("Centro Histórico de %s", city), fmt.Sprintf("Bairro Operário de %s", city),
			fmt.Sprintf("Vila Nova de %s", city), fmt.Sprintf("Jardim América de %s", city), fmt.Sprintf("Bairro São José de %s", city),
			fmt.Sprintf("Parque das Flores de %s", city), fmt.Sprintf("Bairro Universitário de %s", city), fmt.Sprintf("Bairro Industrial de %s", city),
			fmt.Sprintf("Bela Vista de %s", city), fmt.Sprintf("Alto da Colina de %s", city), fmt.Sprintf("Bairro dos Ipês de %s", city),
			fmt.Sprintf("Avenida Central de %s", city), fmt.Sprintf("Parque Ecológico de %s", city), fmt.Sprintf("Mirante Verde de %s", city),
			fmt.Sprintf("Bairro Imperial de %s", city), fmt.Sprintf("Jardim Botânico de %s", city), fmt.Sprintf("Vila Mariana de %s", city),
			fmt.Sprintf("Condomínio dos Lagos de %s", city), fmt.Sprintf("Alphaville de %s", city), fmt.Sprintf("Residencial das Palmeiras de %s", city),
			fmt.Sprintf("Avenida das Nações de %s", city), fmt.Sprintf("Avenida Beira-Rio de %s", city),
		},
		stations: [4]string{
			fmt.Sprintf("Terminal Rodoviário Central de %s", city),
			fmt.Sprintf("Estação Ferroviária de %s", city),
			fmt.Sprintf("Terminal Metropolitano de %s", city),
			fmt.Sprintf("Aeroporto Regional de %s", city),
		},
		utilities: [2]string{
			fmt.Sprintf("Companhia de Energia de %s", city),
			fmt.Sprintf("Companhia de Água e Esgoto de %s", city),
		},
		candidates: [4]string{
			fmt.Sprintf("Praça Central de %s", city),
			fmt.Sprintf("Centro Histórico de %s", city),
			fmt.Sprintf("Parque Ecológico de %s", city),
			fmt.Sprintf("Mirante da Cidade de %s", city),
		},
	}
}
