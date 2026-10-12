package app

import (
	"context"
	"errors"
	"log/slog"

	"ai-inbox/internal/domain"
)

// TenantService: owner onboarding, business context, policy flags,
// channel connection and webhook restore after restart.
type TenantService struct {
	tenants  domain.TenantRepository
	leads    domain.LeadRepository
	vault    domain.CredentialVault
	registry domain.ChannelRegistry
	log      *slog.Logger
}

func NewTenantService(tenants domain.TenantRepository, leads domain.LeadRepository, vault domain.CredentialVault, registry domain.ChannelRegistry, log *slog.Logger) *TenantService {
	return &TenantService{tenants: tenants, leads: leads, vault: vault, registry: registry, log: log}
}

func (s *TenantService) EnsureTenant(ctx context.Context, ownerChatID int64, name string) (*domain.Tenant, error) {
	return s.tenants.EnsureTenant(ctx, ownerChatID, name)
}

func (s *TenantService) SetBusinessCard(ctx context.Context, ownerChatID int64, text string) error {
	tenant, err := s.tenants.GetTenantByOwnerChat(ctx, ownerChatID)
	if err != nil {
		return err
	}
	return s.tenants.SetBusinessCard(ctx, tenant.ID, text)
}

func (s *TenantService) SetPolicyFlag(ctx context.Context, ownerChatID int64, key string, on bool) error {
	tenant, err := s.tenants.GetTenantByOwnerChat(ctx, ownerChatID)
	if err != nil {
		return err
	}
	return s.tenants.SetPolicyFlag(ctx, tenant.ID, key, on)
}

func (s *TenantService) Policy(ctx context.Context, ownerChatID int64) (domain.Policy, error) {
	tenant, err := s.tenants.GetTenantByOwnerChat(ctx, ownerChatID)
	if err != nil {
		return domain.Policy{}, err
	}
	return domain.ParsePolicy(tenant.Policy), nil
}

func (s *TenantService) Stats(ctx context.Context, ownerChatID int64) (map[string]int, error) {
	tenant, err := s.tenants.GetTenantByOwnerChat(ctx, ownerChatID)
	if err != nil {
		return nil, err
	}
	return s.leads.ConversationStats(ctx, tenant.ID)
}

// ConnectTelegramChannel stores the channel and registers its webhook.
// The token is validated by the caller (delivery layer) via GetMe.
func (s *TenantService) ConnectTelegramChannel(ctx context.Context, ownerChatID int64, token, username string) (*domain.Channel, error) {
	tenant, err := s.tenants.GetTenantByOwnerChat(ctx, ownerChatID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	enc, err := s.vault.Encrypt([]byte(token))
	if err != nil {
		return nil, err
	}
	ch, err := s.tenants.CreateChannel(ctx, tenant.ID, "telegram", enc, username)
	if err != nil {
		return nil, err
	}
	if err := s.registry.Register(ctx, ch.ID, token); err != nil {
		_ = s.tenants.SetChannelStatus(ctx, ch.ID, "error")
		return nil, err
	}
	return ch, nil
}

// RestoreChannels re-registers webhooks for all active telegram channels.
// setWebhook is idempotent, so this is safe on every restart.
func (s *TenantService) RestoreChannels(ctx context.Context) error {
	channels, err := s.tenants.ListChannels(ctx, "telegram", "active")
	if err != nil {
		return err
	}
	for _, ch := range channels {
		token, err := s.vault.Decrypt(ch.Credentials)
		if err != nil {
			s.log.Error("restore: decrypt credentials", "channel", ch.ID, "err", err)
			continue
		}
		if err := s.registry.Register(ctx, ch.ID, string(token)); err != nil {
			s.log.Error("restore: register webhook", "channel", ch.ID, "err", err)
			_ = s.tenants.SetChannelStatus(ctx, ch.ID, "error")
		}
	}
	return nil
}
