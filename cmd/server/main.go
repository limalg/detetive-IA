package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"detetive-ia/internal/db"
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

	handler := web.FileServer()

	server := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	go func() {
		log.Printf("🕵️ Detetive IA rodando em http://localhost:%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Erro no servidor HTTP: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Encerrando servidor Detetive IA...")
}
