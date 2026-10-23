package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"detetive-ia/internal/auth"
	"detetive-ia/internal/banco3d"
	"detetive-ia/internal/db"
	"detetive-ia/internal/game"
	"detetive-ia/internal/room"
	"detetive-ia/internal/server"
	"detetive-ia/web"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = filepath.Join("data", "detetive.db")
	}

	dbDir := filepath.Dir(dbPath)
	if dbDir != "." && dbDir != "" {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			log.Printf("Aviso: falha ao criar diretório para banco de dados: %v", err)
		}
	}

	database, err := db.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Erro fatal: falha ao inicializar banco de dados SQLite: %v", err)
	}
	defer database.Close()

	if err := db.RunMigrations(database); err != nil {
		log.Fatalf("Erro fatal: falha ao executar migrações do banco: %v", err)
	}

	authService := auth.New(database)
	ensureAdmin(authService, dbDir)

	// Carregar configurações de IA do banco (padrão da nota credencias)
	var aiURL, aiKey, aiModel, aiProvider string
	_ = database.QueryRow("SELECT value FROM system_settings WHERE key = 'ai_url'").Scan(&aiURL)
	_ = database.QueryRow("SELECT value FROM system_settings WHERE key = 'ai_api_key'").Scan(&aiKey)
	_ = database.QueryRow("SELECT value FROM system_settings WHERE key = 'ai_model'").Scan(&aiModel)
	_ = database.QueryRow("SELECT value FROM system_settings WHERE key = 'ai_provider'").Scan(&aiProvider)

	aiClient := game.NewUniversalAIClient(game.AIConfig{
		URL:      aiURL,
		APIKey:   aiKey,
		Model:    aiModel,
		Provider: aiProvider,
	})

	gameService := game.New(database, aiClient)
	if _, err := gameService.SeedInitialCase(); err != nil {
		log.Printf("Aviso: falha ao semear caso inicial: %v", err)
	}
	if err := game.SeedDetectiveNightCases(database); err != nil {
		log.Printf("Aviso: falha ao semear catálogo Detective Night: %v", err)
	}

	roomManager, err := room.NewManager(database)
	if err != nil {
		log.Printf("Aviso: falha ao inicializar Room Manager: %v", err)
	}
	weaver := game.NewWeaver(database)

	cfg := server.Config{
		Addr:        ":" + port,
		DB:          database,
		AuthService: authService,
		GameService: gameService,
		RoomManager: roomManager,
		Weaver:      weaver,
		StaticFS:    web.FileServer(),
	}

	// hub criado aqui para poder salvar periodicamente e no shutdown (rebuild/restart não perde salas)
	cfg.Banco3DHub = banco3d.NewRoomHub()
	srv := server.NewServer(cfg)

	go func() {
		log.Printf("🕵️ Detetive IA rodando em http://localhost:%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Erro no servidor HTTP: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	autosave := time.NewTicker(30 * time.Second)
	defer autosave.Stop()
wait:
	for {
		select {
		case <-autosave.C:
			_ = cfg.Banco3DHub.SaveToDisk()
		case <-quit:
			break wait
		}
	}

	fmt.Println("Encerrando servidor Detetive IA...")
	if err := cfg.Banco3DHub.SaveToDisk(); err != nil {
		log.Printf("Aviso: falha ao salvar salas do Banco 3D no shutdown: %v", err)
	}
}

// ensureAdmin cria o admin inicial a partir de ADMIN_USER/ADMIN_PASSWORD. Sem senha no env,
// gera uma aleatória, grava em <dados>/admin_password.txt (0600) e loga apenas o caminho.
func ensureAdmin(a *auth.Service, dataDir string) {
	user := os.Getenv("ADMIN_USER")
	if user == "" {
		user = "admin"
	}
	pass := os.Getenv("ADMIN_PASSWORD")
	generated := pass == ""
	if generated {
		b := make([]byte, 18)
		if _, err := rand.Read(b); err != nil {
			log.Fatalf("Erro fatal: falha ao gerar senha do admin: %v", err)
		}
		pass = base64.RawURLEncoding.EncodeToString(b)
	}
	created, err := a.EnsureAdminUser(user, pass)
	if err != nil {
		log.Printf("Aviso: falha ao criar admin: %v", err)
		return
	}
	if created == nil || !generated {
		return // já existia admin, ou senha veio do env
	}
	path := filepath.Join(dataDir, "admin_password.txt")
	if err := os.WriteFile(path, []byte(pass+"\n"), 0600); err != nil {
		log.Fatalf("Erro fatal: admin criado mas senha não pôde ser gravada em %s: %v", path, err)
	}
	log.Printf("Admin %q criado com senha aleatória; senha gravada em %s", user, path)
}
