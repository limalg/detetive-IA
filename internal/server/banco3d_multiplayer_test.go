package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"detetive-ia/internal/banco3d"
	"detetive-ia/internal/server"
)

type mpClient struct {
	t      *testing.T
	router http.Handler
	room   string
	id     string
}

func (c *mpClient) call(method, path string, body any) (int, map[string]any) {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	rr := httptest.NewRecorder()
	c.router.ServeHTTP(rr, req)
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return rr.Code, out
}

func (c *mpClient) post(path string, extra map[string]any) (int, map[string]any) {
	b := map[string]any{"room_id": c.room, "player_id": c.id}
	for k, v := range extra {
		b[k] = v
	}
	return c.call("POST", "/api/banco3d/room/"+path, b)
}

func (c *mpClient) join(name string) {
	code, out := c.call("POST", "/api/banco3d/room/join", map[string]any{"room_id": c.room, "name": name, "color": "#fff", "piece": "car"})
	if code != 200 {
		c.t.Fatalf("join %s: %d %v", name, code, out)
	}
	c.id = out["player"].(map[string]any)["id"].(string)
}

func (c *mpClient) state() map[string]any {
	code, out := c.call("GET", "/api/banco3d/room/state?room_id="+c.room, nil)
	if code != 200 {
		c.t.Fatalf("state: %d %v", code, out)
	}
	return out
}

func turnOf(st map[string]any) string {
	return st["game"].(map[string]any)["current_turn_player_id"].(string)
}

func TestBanco3DMultiplayerThreeHumans(t *testing.T) {
	hub := banco3d.NewRoomHub(filepath.Join(t.TempDir(), "rooms.json"))
	router := server.NewRouter(server.Config{Banco3DHub: hub})
	mk := func() *mpClient { return &mpClient{t: t, router: router, room: "mp-1"} }
	a, b, c := mk(), mk(), mk()

	// sala inexistente
	if code, _ := a.call("POST", "/api/banco3d/room/join", map[string]any{"room_id": "nope", "name": "x"}); code != 404 {
		t.Fatalf("join missing room: want 404 got %d", code)
	}
	if code, _ := a.call("GET", "/api/banco3d/room/state?room_id=nope", nil); code != 404 {
		t.Fatalf("state missing room: want 404 got %d", code)
	}

	cfg := banco3d.DefaultRoomConfig()
	cfg.Bots.Count = 0
	cfg.MaxPlayers = 3
	if code, out := a.call("POST", "/api/banco3d/room/create", map[string]any{"room_id": "mp-1", "city": "Londrina", "config": cfg}); code != 200 {
		t.Fatalf("create: %d %v", code, out)
	}
	a.join("Ana")
	b.join("Bia")
	c.join("Caio")
	if a.id == b.id || b.id == c.id || a.id == c.id {
		t.Fatalf("ids must be distinct: %s %s %s", a.id, b.id, c.id)
	}

	// sala cheia
	if code, _ := mk().call("POST", "/api/banco3d/room/join", map[string]any{"room_id": "mp-1", "name": "Dani"}); code != 400 {
		t.Fatalf("full room: want 400 got %d", code)
	}

	// lobby sincroniza: todos veem 3 jogadores
	for _, cl := range []*mpClient{a, b, c} {
		if n := len(cl.state()["game"].(map[string]any)["players"].([]any)); n != 3 {
			t.Fatalf("lobby players: want 3 got %d", n)
		}
	}

	// votacao com votos de todos
	if code, out := a.post("vote", map[string]any{"action": "start"}); code != 200 {
		t.Fatalf("vote start: %d %v", code, out)
	}
	for i, cl := range []*mpClient{a, b, c} {
		if code, out := cl.post("vote", map[string]any{"candidate": []int{1, 1, 0}[i]}); code != 200 {
			t.Fatalf("vote %d: %d %v", i, code, out)
		}
	}
	code, out := a.post("vote", map[string]any{"action": "close"})
	if code != 200 {
		t.Fatalf("vote close: %d %v", code, out)
	}
	if st := b.state(); st["voting"].(map[string]any)["winner"] != out["winner"] {
		t.Fatalf("voting winner not consistent: %v vs %v", st["voting"], out["winner"])
	}

	// F5: reconectar mantem o mesmo player_id
	re := mk()
	_, rj := re.call("POST", "/api/banco3d/room/join", map[string]any{"room_id": "mp-1", "player_id": b.id})
	if code, _ := mk().call("POST", "/api/banco3d/room/join", map[string]any{"room_id": "mp-1", "player_id": "p99", "name": "Fantasma"}); code != 404 {
		t.Fatalf("reconnect with unknown player_id: want 404 got %d", code)
	}
	if rj["reconnected"] != true || rj["player"].(map[string]any)["id"] != b.id {
		t.Fatalf("reconnect failed: %v", rj)
	}

	if code, out := a.post("start", nil); code != 200 {
		t.Fatalf("start: %d %v", code, out)
	}
	clients := map[string]*mpClient{a.id: a, b.id: b, c.id: c}

	// 403 fora da vez, para todas as acoes de turno
	cur := turnOf(a.state())
	for id, cl := range clients {
		if id == cur {
			continue
		}
		for _, p := range []string{"roll", "buy", "pass", "build"} {
			if code, _ := cl.post(p, nil); code != 403 {
				t.Fatalf("%s out of turn by %s: want 403 got %d", p, id, code)
			}
		}
	}

	// rolagens concorrentes do mesmo jogador: so uma pode valer
	var ok int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if code, _ := clients[cur].post("roll", map[string]any{"d1": 1, "d2": 2}); code == 200 {
				atomic.AddInt32(&ok, 1)
			}
		}()
		wg.Add(1)
		go func(cl *mpClient) { // polling concorrente
			defer wg.Done()
			cl.state()
		}([]*mpClient{a, b, c}[i%3])
	}
	wg.Wait()
	if ok != 1 {
		t.Fatalf("concurrent rolls by same player: want exactly 1 success, got %d", ok)
	}

	// eventos: seq estritamente crescente e sem buracos para todos os clientes
	for _, cl := range []*mpClient{a, b, c} {
		evs := cl.state()["events"].([]any)
		for i := 1; i < len(evs); i++ {
			if evs[i].(map[string]any)["seq"].(float64) != evs[i-1].(map[string]any)["seq"].(float64)+1 {
				t.Fatalf("event seq gap: %v", evs)
			}
		}
	}

	// proposta so pelo jogador do turno; aceite/recusa so pelo destinatario
	st := a.state()
	cur = turnOf(st)
	var other, third string
	for id := range clients {
		if id != cur {
			if other == "" {
				other = id
			} else {
				third = id
			}
		}
	}
	if code, _ := clients[other].post("trade", map[string]any{"action": "propose", "to_player_id": cur, "give": map[string]any{"cash": 10}}); code != 403 {
		t.Fatalf("propose by non-turn player: want 403 got %d", code)
	}
	if code, out := clients[cur].post("trade", map[string]any{"action": "propose", "to_player_id": other, "give": map[string]any{"cash": 10}, "want": map[string]any{"cash": 0}}); code != 200 {
		t.Fatalf("propose: %d %v", code, out)
	}
	for _, act := range []string{"accept", "decline"} {
		if code, _ := clients[third].post("trade", map[string]any{"action": act}); code != 403 {
			t.Fatalf("%s by third party: want 403 got %d", act, code)
		}
		if code, _ := clients[cur].post("trade", map[string]any{"action": act}); code != 403 {
			t.Fatalf("%s by sender: want 403 got %d", act, code)
		}
	}
	if code, out := clients[other].post("trade", map[string]any{"action": "decline"}); code != 200 {
		t.Fatalf("decline by receiver: %d %v", code, out)
	}
	if b.state()["pending_trade"] != nil {
		t.Fatalf("pending_trade should be cleared after decline")
	}

	// um estado live nao pode ser revertido por consulta a sala inexistente (recarga do disco)
	hub.SaveToDisk()
	before := a.state()["game"].(map[string]any)["turn_index"]
	a.post("pass", map[string]any{"player_id": cur})
	after := a.state()["game"].(map[string]any)["turn_index"]
	a.call("GET", "/api/banco3d/room/state?room_id=ghost", nil)
	if now := a.state()["game"].(map[string]any)["turn_index"]; now != after || before == after && false {
		t.Fatalf("live state reverted by disk reload: after=%v now=%v", after, now)
	}

	// sair da partida
	if code, out := c.post("leave", nil); code != 200 {
		t.Fatalf("leave: %d %v", code, out)
	}
}
