// Command server is the composition root: it wires together all layers
// (domain <- app <- infra/delivery) and runs the process.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	"ai-inbox/internal/app"
	"ai-inbox/internal/config"
	"ai-inbox/internal/delivery/httpapi"
	"ai-inbox/internal/delivery/ownerbot"
	"ai-inbox/internal/domain"
	"ai-inbox/internal/infra/openrouter"
	"ai-inbox/internal/infra/postgres"
	"ai-inbox/internal/infra/pgqueue"
	"ai-inbox/internal/infra/telegram"
	"ai-inbox/internal/infra/vault"
)

// channelTokens implements telegram.TokenSource.
type channelTokens struct {
	tenants domain.TenantRepository
	vault   domain.CredentialVault
}

func (s channelTokens) Token(ctx context.Context, channelID uuid.UUID) (string, error) {
	ch, err := s.tenants.GetChannel(ctx, channelID)
	if err != nil {
		return "", err
	}
	b, err := s.vault.Decrypt(ch.Credentials)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := config.Load()

	// --- infrastructure layer ---

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := postgres.Migrate(ctx, pool); err != nil {
		log.Error("migrate", "err", err)
		os.Exit(1)
	}

	tenantRepo := postgres.NewTenantRepo(pool)
	leadRepo := postgres.NewLeadRepo(pool)
	outboxRepo := postgres.NewOutboxRepo(pool)
	usageRepo := postgres.NewUsageRepo(pool)
	queue := pgqueue.New(pool)
	credVault := vault.New(cfg.AppSecret)
	registry := telegram.NewRegistry(cfg.PublicBaseURL)
	sender := telegram.NewSender(channelTokens{tenants: tenantRepo, vault: credVault})

	var llm domain.LLMQualifier
	if cfg.LLMAPIKey == "" {
		log.Warn("LLM_API_KEY not set, using mock LLM client")
		llm = openrouter.NewMock()
	} else {
		llm = openrouter.New(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel, cfg.LLMFallbackModel)
	}

	// --- application layer ---

	ingestUC := app.NewIngest(leadRepo, tenantRepo, queue, log)
	qualifyUC := app.NewQualify(leadRepo, outboxRepo, llm, usageRepo, log)
	deliverUC := app.NewDeliver(leadRepo, outboxRepo, sender, log)
	ownerUC := app.NewOwnerActions(leadRepo, outboxRepo)
	tenantUC := app.NewTenantService(tenantRepo, leadRepo, credVault, registry, log)
	workers := app.NewWorkers(queue, outboxRepo, qualifyUC, deliverUC, log)

	// --- delivery layer ---

	if cfg.OwnerBotToken == "" {
		log.Error("OWNER_BOT_TOKEN is required")
		os.Exit(1)
	}
	ownerBot, err := ownerbot.New(cfg.OwnerBotToken, ownerUC, tenantUC, leadRepo, log)
	if err != nil {
		log.Error("owner bot", "err", err)
		os.Exit(1)
	}
	ingestUC.SetNotifier(ownerBot)
	qualifyUC.SetNotifier(ownerBot)

	// --- run ---

	if err := tenantUC.RestoreChannels(ctx); err != nil {
		log.Error("restore channels", "err", err)
	}
	ownerBot.Start()
	go func() {
		if err := workers.Run(ctx); err != nil {
			log.Error("workers", "err", err)
		}
	}()

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: httpapi.New(ingestUC)}
	go func() {
		log.Info("http server listening", "addr", cfg.HTTPAddr, "public_base_url", cfg.PublicBaseURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", "err", err)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	ownerBot.Stop()
}
