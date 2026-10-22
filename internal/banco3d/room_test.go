package banco3d

import (
	"testing"
	"time"
)

func TestRoomLifecycleAndEvents(t *testing.T) {
	hub := NewRoomHub()
	room, err := hub.CreateRoom("SP-123", "São Paulo")
	if err != nil || room == nil {
		t.Fatalf("falha ao criar sala: %v", err)
	}
	if room.City != "São Paulo" {
		t.Errorf("cidade incorreta: %s", room.City)
	}

	// 1. Jogadores entram na sala
	p1, err := room.JoinPlayer("Lucas", "#3b82f6", "car")
	if err != nil {
		t.Fatalf("falha ao entrar p1: %v", err)
	}
	p2, err := room.JoinPlayer("Ana", "#ef4444", "hat")
	if err != nil {
		t.Fatalf("falha ao entrar p2: %v", err)
	}

	if len(room.Game.Players) != 2 {
		t.Errorf("esperado 2 jogadores na sala, recebido %d", len(room.Game.Players))
	}

	// 2. Iniciar votação relâmpago de 15s
	if err := room.StartVoting(); err != nil {
		t.Fatalf("falha ao iniciar votação: %v", err)
	}
	if room.Voting == nil || len(room.Voting.Candidates) != 4 {
		t.Fatalf("esperado sessão de votação com 4 candidatos")
	}

	// 3. Votar e concluir votação
	if err := room.CastVote(p1.ID, 0); err != nil {
		t.Fatalf("falha ao votar: %v", err)
	}
	winner, err := room.ConcludeVoting()
	if err != nil {
		t.Fatalf("falha ao concluir votação: %v", err)
	}
	if winner == "" {
		t.Errorf("vencedor não pode ser vazio")
	}

	// 4. Iniciar partida e validar turno
	if err := room.StartGame(); err != nil {
		t.Fatalf("falha ao iniciar partida: %v", err)
	}
	if !room.Game.IsStarted {
		t.Errorf("jogo deveria estar iniciado")
	}

	// 5. Reconexão de jogador caído
	reconnectedP1, err := room.ReconnectPlayer(p1.ID)
	if err != nil {
		t.Fatalf("falha ao reconectar p1: %v", err)
	}
	if reconnectedP1.ID != p1.ID || reconnectedP1.Name != "Lucas" {
		t.Errorf("dados do jogador reconectado divergem: %+v", reconnectedP1)
	}

	// 6. Testar timeout de turno de 30s
	room.Game.TurnStartTime = time.Now().Add(-35 * time.Second)
	room.CheckTurnTimeout()
	if room.Game.CurrentTurnPlayerID != p2.ID {
		t.Errorf("auto-pass deveria ter passado a vez para p2 por timeout, atual: %s", room.Game.CurrentTurnPlayerID)
	}
}

func TestTradeAuthorizationAndBotAutoAccept(t *testing.T) {
	hub := NewRoomHub()
	room, _ := hub.CreateRoom("TRADE-TEST", "Curitiba")
	p1, _ := room.JoinPlayer("Lucas", "#3b82f6", "car") // p1 (humano)
	p2, _ := room.JoinPlayer("Ana", "#ef4444", "hat")   // p2 (humano)
	_ = room.StartGame()

	// 1. Validar que p1 e p2 são humanos (IsBot == false)
	if p1.IsBot {
		t.Errorf("p1 não deveria ser bot")
	}
	if p2.IsBot {
		t.Errorf("p2 não deveria ser bot")
	}

	// 2. Proposta de troca de p1 para p2
	proposal := TradeProposal{
		ID:             "trade-1",
		SenderID:       p1.ID,
		ReceiverID:     p2.ID,
		OfferedTiles:   []int{},
		OfferedCash:    100,
		RequestedTiles: []int{},
		RequestedCash:  50,
	}
	_ = room.ProposeTrade(proposal)

	// 3. Tentativa de aceitação pelo próprio remetente (p1) -> DEVE FALHAR
	if err := room.AcceptTrade(p1.ID); err == nil {
		t.Errorf("remetente (p1) não pode aceitar a própria proposta")
	}

	// 4. Tentativa de recusa por terceiro desconhecido -> DEVE FALHAR
	if err := room.DeclineTrade("hacker_p99"); err == nil {
		t.Errorf("jogador não participante não pode recusar a proposta alheia")
	}

	// 5. Destinatário legítimo (p2) aceita -> DEVE PASSAR
	if err := room.AcceptTrade(p2.ID); err != nil {
		t.Fatalf("destinatário legítimo (p2) deveria conseguir aceitar a troca: %v", err)
	}

	// 6. Teste de proposta para BOT com auto-aceite
	roomSolo, _ := hub.CreateRoom("SOLO-BOT-TEST", "Curitiba")
	soloHuman, _ := roomSolo.JoinPlayer("Marlowe", "#3b82f6", "car")
	_ = roomSolo.StartGame() // auto-adiciona bots

	var botPlayer *Player
	for _, p := range roomSolo.Game.Players {
		if p.ID != soloHuman.ID {
			botPlayer = p
			break
		}
	}
	if botPlayer == nil || !botPlayer.IsBot {
		t.Fatalf("bot não encontrado ou IsBot false")
	}

	// Proposta para o bot
	botProposal := TradeProposal{
		ID:             "trade-bot-1",
		SenderID:       soloHuman.ID,
		ReceiverID:     botPlayer.ID,
		OfferedTiles:   []int{},
		OfferedCash:    50,
		RequestedTiles: []int{},
		RequestedCash:  0,
	}
	_ = roomSolo.ProposeTrade(botProposal)

	// Bot legítimo aceita
	if err := roomSolo.AcceptTrade(botPlayer.ID); err != nil {
		t.Fatalf("bot deveria conseguir aceitar troca: %v", err)
	}
}
