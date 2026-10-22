package banco3d

import (
	"strings"
	"testing"
)

func TestCityBoardGeneratorOffline(t *testing.T) {
	// 1. Cidade com catálogo local (Curitiba)
	board, candidates, err := GenerateCityBoard("Curitiba, PR", "")
	if err != nil {
		t.Fatalf("geração falhou: %v", err)
	}
	if len(board.Tiles) != 40 {
		t.Errorf("esperado 40 casas, recebido %d", len(board.Tiles))
	}
	if len(candidates) != 4 {
		t.Errorf("esperado exatamente 4 candidatos a ponto icônico, recebido %d", len(candidates))
	}

	// Verificar se nomes reais de Curitiba estão no tabuleiro
	found := false
	for _, tile := range board.Tiles {
		if strings.Contains(tile.Name, "Batel") || strings.Contains(tile.Name, "Largo da Ordem") || strings.Contains(tile.Name, "Jardim Botânico") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("esperado ruas reais de Curitiba no tabuleiro")
	}

	// 2. Cidade arbitrária/desconhecida (fallback procedural)
	b2, c2, err := GenerateCityBoard("Xique-Xique, BA", "")
	if err != nil {
		t.Fatalf("geração procedural falhou: %v", err)
	}
	if len(b2.Tiles) != 40 {
		t.Errorf("esperado 40 casas para cidade procedural, recebido %d", len(b2.Tiles))
	}
	if len(c2) != 4 {
		t.Errorf("esperado 4 candidatos para cidade procedural, recebido %d", len(c2))
	}
}
