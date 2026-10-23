package banco3d

import (
	"testing"
)

func TestBoardStructure(t *testing.T) {
	b := NewStandardBoard("Curitiba, PR", nil)
	if len(b.Tiles) != 40 {
		t.Fatalf("esperado 40 casas, recebido %d", len(b.Tiles))
	}

	// Cantos Business Tour: 0=START, 10=TRAVEL, 20=JAIL, 30=SPECIAL_TAX
	if b.Tiles[0].Type != TileStart {
		t.Errorf("casa 0 deve ser START, recebido %v", b.Tiles[0].Type)
	}
	if b.Tiles[10].Type != TileTravel {
		t.Errorf("casa 10 deve ser TRAVEL (Viagem), recebido %v", b.Tiles[10].Type)
	}
	if b.Tiles[20].Type != TileJail {
		t.Errorf("casa 20 deve ser JAIL (Ilha Perdida), recebido %v", b.Tiles[20].Type)
	}
	if b.Tiles[30].Type != TileSpecialTax {
		t.Errorf("casa 30 deve ser SPECIAL_TAX (Imposto Especial), recebido %v", b.Tiles[30].Type)
	}

	// Contagem de tipos
	var props, stations, utils, taxes, luckCount int
	for _, tile := range b.Tiles {
		switch tile.Type {
		case TileProperty:
			props++
		case TileStation:
			stations++
		case TileUtility:
			utils++
		case TileTax:
			taxes++
		case TileLuck:
			luckCount++
		}
	}

	if props != 22 {
		t.Errorf("esperado 22 propriedades, recebido %d", props)
	}
	if stations != 4 {
		t.Errorf("esperado 4 estações, recebido %d", stations)
	}
	if utils != 2 {
		t.Errorf("esperado 2 utilitários, recebido %d", utils)
	}
	if taxes != 2 {
		t.Errorf("esperado 2 impostos, recebido %d", taxes)
	}
	if luckCount != 6 {
		t.Errorf("esperado 6 casas de Sorte ou Revés, recebido %d", luckCount)
	}

	// Sorte ou Revés unificadas espalhadas pelos 4 lados
	luckIndices := []int{2, 7, 17, 22, 33, 36}
	for _, idx := range luckIndices {
		if b.Tiles[idx].Type != TileLuck {
			t.Errorf("casa %d deveria ser TileLuck, obtido %s", idx, b.Tiles[idx].Type)
		}
		if b.Tiles[idx].Name != "Sorte ou Revés" {
			t.Errorf("casa %d deveria chamar 'Sorte ou Revés', obtido '%s'", idx, b.Tiles[idx].Name)
		}
	}

	// Validação de ordem BARATO -> CARO (monotônica crescente entre grupos de cor)
	lastPrice := 0
	for i := 0; i < len(b.Tiles); i++ {
		if b.Tiles[i].Type == TileProperty {
			if b.Tiles[i].Price < lastPrice {
				t.Errorf("ordem BARATO->CARO violada na casa %d (%s): preço %d < anterior %d",
					i, b.Tiles[i].Name, b.Tiles[i].Price, lastPrice)
			}
			lastPrice = b.Tiles[i].Price
		}
	}

	// Rainha na casa 39
	queen := b.Tiles[39]
	if !queen.IsQueen {
		t.Errorf("casa 39 deve ser IsQueen=true")
	}
	if queen.Price != 400 {
		t.Errorf("casa 39 deve custar 400, recebido %d", queen.Price)
	}
	if queen.Rent[0] != 50 {
		t.Errorf("casa 39 aluguel base deve ser 50, recebido %d", queen.Rent[0])
	}
}
