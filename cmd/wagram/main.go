/*
 * ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 *  Wagram — Entry Point
 * ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 *
 *  Bootstraps all layers of the application:
 *    1. Loads configuration from environment
 *    2. Opens SQLite repository for chat mappings
 *    3. Initializes WhatsApp (whatsmeow) service
 *    4. Initializes Telegram (gotgbot) service
 *    5. Wires the Wagram service (WA ↔ TG relay)
 *    6. Registers command handlers
 *    7. Starts long-polling and waits for shutdown
 */
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"telegram-wa/internal/config"
	"telegram-wa/internal/handler"
	"telegram-wa/internal/repository"
	"telegram-wa/internal/service"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting wagram...")

	/* ── Configuration ───────────────────────── */
	cfg := config.Load()
	if cfg.TelegramBotToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	/* ── Repository (wagram.db) ──────────────── */
	repo, err := repository.NewSQLiteRepository(cfg.WagramDBPath)
	if err != nil {
		log.Fatalf("init repository: %v", err)
	}
	defer repo.Close()

	/* ── WhatsApp Service (wa_session.db) ─────── */
	waSvc, err := service.NewWhatsAppService(cfg.WASessionDBPath)
	if err != nil {
		log.Fatalf("init whatsapp: %v", err)
	}
	defer waSvc.Disconnect()

	/* ── Telegram Service ────────────────────── */
	tgSvc, err := service.NewTelegramService(cfg.TelegramBotToken)
	if err != nil {
		log.Fatalf("init telegram: %v", err)
	}

	/* ── Wagram Service (wires WA ↔ TG) ──────── */
	wagramSvc := service.NewWagramService(repo, waSvc, tgSvc)

	/* ── Command & Message Handlers ──────────── */
	handler.Register(tgSvc.Dispatcher, waSvc, repo, wagramSvc)

	/* ── Start Services ──────────────────────── */
	if err := tgSvc.Start(); err != nil {
		log.Fatalf("start telegram: %v", err)
	}
	defer tgSvc.Stop()

	_ = waSvc.Connect()

	log.Println("Wagram is running. Press Ctrl+C to exit.")

	/* ── Graceful Shutdown ────────────────────── */
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	log.Println("Shutting down...")
}
