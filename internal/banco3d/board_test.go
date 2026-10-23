package banco3d

import (
	"testing"
)

func TestBoardStructure(t *testing.T) {
	b := NewStandardBoard("Curitiba, PR", nil)
	if len(b.Tiles) != 40 {
		t.Fatalf("esperado 40 casas, recebido %d", len(b.Tiles))
	}

	// Cantos canônicos
	if b.Tiles[0].Type != TileGo {
		t.Errorf("casa 0 deve ser GO, recebido %v", b.Tiles[0].Type)
	}
	if b.Tiles[10].Type != TileJail {
		t.Errorf("casa 10 deve ser Prisão, recebido %v", b.Tiles[10].Type)
	}
	if b.Tiles[20].Type != TileFreeParking {
		t.Errorf("casa 20 deve ser FreeParking, recebido %v", b.Tiles[20].Type)
	}

	// Contagem de tipos
	var props, stations, utils, taxes int
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
