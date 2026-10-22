package banco3d

import (
	"testing"
)

func TestGameMovementAndBuy(t *testing.T) {
	board := NewStandardBoard("São Paulo", nil)
	game := NewGame("room-1", board)
	p1, err := game.AddPlayer("Lucas", "#3b82f6", "car")
	if err != nil {
		t.Fatalf("erro ao adicionar p1: %v", err)
	}
	p2, err := game.AddPlayer("Ana", "#ef4444", "hat")
	if err != nil {
		t.Fatalf("erro ao adicionar p2: %v", err)
	}

	if err := game.Start(); err != nil {
		t.Fatalf("erro ao iniciar jogo: %v", err)
	}

	// 1. Simular jogada de dados para casa 1 (Marrom A, preço 60)
	res, err := game.RollDice(p1.ID, 0, 1)
	if err != nil {
		t.Fatalf("erro ao rolar dados: %v", err)
	}
	if res.NewPosition != 1 {
		t.Errorf("esperado posicao 1, recebido %d", res.NewPosition)
	}
	if p1.Position != 1 {
		t.Errorf("jogador posicao esperado 1, recebido %d", p1.Position)
	}

	// 2. Comprar propriedade
	initialMoney := p1.Money
	if err := game.BuyProperty(p1.ID); err != nil {
		t.Fatalf("erro ao comprar propriedade: %v", err)
	}
	if p1.Money != initialMoney-60 {
		t.Errorf("dinheiro esperado %d, recebido %d", initialMoney-60, p1.Money)
	}
	if game.Board.Tiles[1].OwnerID != p1.ID {
		t.Errorf("dono da casa 1 deve ser p1, recebido %s", game.Board.Tiles[1].OwnerID)
	}

	// 3. Passar turno
	if err := game.PassTurn(p1.ID); err != nil {
		t.Fatalf("erro ao passar turno: %v", err)
	}
	if game.CurrentTurnPlayerID != p2.ID {
		t.Errorf("vez deve ser do p2, recebido %s", game.CurrentTurnPlayerID)
	}

	// 4. Jogador 2 cai na casa 1 de p1 e paga aluguel (base 2)
	p2Money := p2.Money
	p1Money := p1.Money
	_, err = game.RollDice(p2.ID, 0, 1)
	if err != nil {
		t.Fatalf("erro ao rolar dados p2: %v", err)
	}
	if p2.Money != p2Money-2 {
		t.Errorf("p2 dinheiro esperado %d, recebido %d", p2Money-2, p2.Money)
	}
	if p1.Money != p1Money+2 {
		t.Errorf("p1 dinheiro esperado %d, recebido %d", p1Money+2, p1.Money)
	}
}

func TestGameGoBonusAndTimeout(t *testing.T) {
	board := NewStandardBoard("Curitiba", nil)
	game := NewGame("room-2", board)
	p1, _ := game.AddPlayer("Lucas", "#3b82f6", "car")
	game.AddPlayer("Bot", "#22c55e", "thimble")
	game.Start()

	// Posicionar jogador na casa 38 e andar 2 casas (cai exatamente no GO)
	p1.Position = 38
	mBefore := p1.Money
	res, err := game.RollDice(p1.ID, 1, 1)
	if err != nil {
		t.Fatalf("falha ao rolar: %v", err)
	}
	if res.NewPosition != 0 {
		t.Errorf("esperado cair no GO (0), recebido %d", res.NewPosition)
	}
	// Passar ou cair exatamente no GO concede R$ 400
	if p1.Money != mBefore+400 {
		t.Errorf("esperado bônus de cair no GO de 400, dinheiro: %d (era %d)", p1.Money, mBefore)
	}

	// Testar auto-pass
	if err := game.AutoPassTurn(); err != nil {
		t.Fatalf("falha no auto-pass: %v", err)
	}
	if game.CurrentTurnPlayerID == p1.ID {
		t.Errorf("auto-pass deveria ter trocado de jogador")
	}
}

func TestGameBuildAndTrade(t *testing.T) {
	board := NewStandardBoard("Curitiba", nil)
	game := NewGame("room-3", board)
	p1, _ := game.AddPlayer("Lucas", "#3b82f6", "car")
	p2, _ := game.AddPlayer("Ana", "#ef4444", "hat")
	game.Start()

	// 1. Dar o grupo Marrom (casas 1 e 2) para p1
	board.Tiles[1].OwnerID = p1.ID
	board.Tiles[2].OwnerID = p1.ID
	p1.Properties = []int{1, 2}

	// 2. Construir casa na casa 1 (custo 50)
	moneyBefore := p1.Money
	if err := game.BuildHouse(p1.ID, 1); err != nil {
		t.Fatalf("falha ao construir casa: %v", err)
	}
	if board.Tiles[1].Houses != 1 {
		t.Errorf("esperado 1 casa construída, recebido %d", board.Tiles[1].Houses)
	}
	if p1.Money != moneyBefore-50 {
		t.Errorf("dinheiro esperado %d, recebido %d", moneyBefore-50, p1.Money)
	}

	// 3. Negociar casa 2 de p1 para p2 por R$ 100
	p1Cash := p1.Money
	p2Cash := p2.Money
	if err := game.TradeProperties(p1.ID, p2.ID, 2, 100); err != nil {
		t.Fatalf("falha no trade: %v", err)
	}
	if board.Tiles[2].OwnerID != p2.ID {
		t.Errorf("dono da casa 2 deve ser p2, recebido %s", board.Tiles[2].OwnerID)
	}
	if p1.Money != p1Cash+100 {
		t.Errorf("p1 dinheiro esperado %d, recebido %d", p1Cash+100, p1.Money)
	}
	if p2.Money != p2Cash-100 {
		t.Errorf("p2 dinheiro esperado %d, recebido %d", p2Cash-100, p2.Money)
	}
}
