// Package ownerbot is the owner-facing Telegram bot: the whole admin UI of
// the MVP. It translates domain errors into user-facing Russian text.
package ownerbot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/telebot.v3"

	"ai-inbox/internal/app"
	"ai-inbox/internal/domain"
	"ai-inbox/internal/infra/telegram"
)

type Bot struct {
	b      *telebot.Bot
	owner  *app.OwnerActions
	tenants *app.TenantService
	store  ownerForwardStore
	log    *slog.Logger
}

// ownerForwardStore persists owner message -> conversation mapping
// (implemented by the lead repository).
type ownerForwardStore interface {
	SaveOwnerForward(ctx context.Context, ownerMsgID int64, conversationID uuid.UUID) error
}

func New(token string, owner *app.OwnerActions, tenants *app.TenantService, forwards ownerForwardStore, log *slog.Logger) (*Bot, error) {
	b, err := telebot.NewBot(telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return nil, fmt.Errorf("create owner bot: %w", err)
	}
	bot := &Bot{b: b, owner: owner, tenants: tenants, store: forwards, log: log}
	b.Handle("/start", bot.onStart)
	b.Handle("/help", bot.onHelp)
	b.Handle("/connect", bot.onConnect)
	b.Handle("/context", bot.onContext)
	b.Handle("/policy", bot.onPolicy)
	b.Handle("/status", bot.onStatus)
	b.Handle(telebot.OnCallback, bot.onCallback)
	b.Handle(telebot.OnText, bot.onText)
	return bot, nil
}

func (b *Bot) Start() { go b.b.Start() }
func (b *Bot) Stop()  { b.b.Stop() }

// --- commands ---

func (b *Bot) onStart(c telebot.Context) error {
	name := "Мой бизнес"
	if args := strings.SplitN(c.Message().Text, " ", 2); len(args) == 2 && args[1] != "" {
		name = args[1]
	}
	if _, err := b.tenants.EnsureTenant(context.Background(), c.Chat().ID, name); err != nil {
		b.log.Error("ensure tenant", "err", err)
	}
	return c.Send("Добро пожаловать в AI-инбокс!\n\n" + helpText)
}

func (b *Bot) onHelp(c telebot.Context) error {
	return c.Send(helpText)
}

func (b *Bot) onConnect(c telebot.Context) error {
	args := strings.Fields(c.Message().Text)
	if len(args) != 2 {
		return c.Send("Создайте бота у @BotFather, затем отправьте:\n/connect <токен_бота>")
	}
	token := args[1]
	ctx := context.Background()
	username, err := telegram.NewBotAPI(token).GetMe(ctx)
	if err != nil {
		return c.Send("Токен недействителен, проверьте его у @BotFather.")
	}
	ch, err := b.tenants.ConnectTelegramChannel(ctx, c.Chat().ID, token, username)
	if err != nil {
		return c.Send("Не удалось подключить: " + userMessage(err))
	}
	return c.Send(fmt.Sprintf("Канал @%s подключён (id %s).\nТеперь заявки из этого бота попадут в инбокс.", ch.ExternalID, ch.ID.String()[:8]))
}

func (b *Bot) onContext(c telebot.Context) error {
	args := strings.SplitN(c.Message().Text, " ", 2)
	if len(args) != 2 || args[1] == "" {
		return c.Send("Опишите бизнес — это попадёт в промпт ИИ:\n/context Ремонт квартир в Москве. Ремонт под ключ от 5000 руб/м2, выезд на замер бесплатно, работаем Пн-Сб 9:00-20:00.")
	}
	if err := b.tenants.SetBusinessCard(context.Background(), c.Chat().ID, args[1]); err != nil {
		return c.Send("Не удалось сохранить: " + userMessage(err))
	}
	return c.Send("Карточка бизнеса сохранена. ИИ будет использовать её для ответов.")
}

func (b *Bot) onPolicy(c telebot.Context) error {
	ctx := context.Background()
	args := strings.Fields(c.Message().Text)
	if len(args) == 1 {
		p, err := b.tenants.Policy(ctx, c.Chat().ID)
		if err != nil {
			return c.Send(userMessage(err))
		}
		return c.Send(fmt.Sprintf("Текущие правила:\nhot_autoreply (автоответ горячим): %v\nwarm_notify (уведомлять о тёплых): %v\ncold_autoreply (автоответ холодным): %v\n\nИзменить: /policy <ключ> <on|off>", p.HotAutoreply, p.WarmNotify, p.ColdAutoreply))
	}
	if len(args) != 3 {
		return c.Send("Формат: /policy <hot_autoreply|warm_notify|cold_autoreply> <on|off>")
	}
	key := args[1]
	switch key {
	case "hot_autoreply", "warm_notify", "cold_autoreply":
	default:
		return c.Send("Неизвестный ключ. Доступны: hot_autoreply, warm_notify, cold_autoreply")
	}
	var on bool
	switch args[2] {
	case "on":
		on = true
	case "off":
		on = false
	default:
		return c.Send("Значение должно быть on или off")
	}
	if err := b.tenants.SetPolicyFlag(ctx, c.Chat().ID, key, on); err != nil {
		return c.Send("Не удалось сохранить: " + userMessage(err))
	}
	state := "выключен"
	if on {
		state = "включён"
	}
	return c.Send(fmt.Sprintf("Правило %s %s.", key, state))
}

func (b *Bot) onStatus(c telebot.Context) error {
	stats, err := b.tenants.Stats(context.Background(), c.Chat().ID)
	if err != nil {
		return c.Send(userMessage(err))
	}
	statusNames := map[string]string{
		"new":      "новые",
		"ai":       "ведёт ИИ",
		"human":    "у владельца",
		"rejected": "отклонённые",
	}
	var sb strings.Builder
	sb.WriteString("Лиды по статусам:\n")
	total := 0
	for _, key := range []string{"new", "ai", "human", "rejected"} {
		sb.WriteString(fmt.Sprintf("%s: %d\n", statusNames[key], stats[key]))
		total += stats[key]
	}
	sb.WriteString(fmt.Sprintf("Всего: %d", total))
	return c.Send(sb.String())
}

// --- plain text: owner replying to a relayed customer message ---

func (b *Bot) onText(c telebot.Context) error {
	m := c.Message()
	if m.ReplyTo == nil {
		return c.Send("Чтобы ответить клиенту, отправьте reply (ответ) на его сообщение.\nДругие команды: /help")
	}
	if err := b.owner.Reply(context.Background(), int64(m.ReplyTo.ID), m.Text); err != nil {
		return c.Send(userMessage(err))
	}
	return c.Send("Отправлено клиенту.")
}

// --- callbacks: take / reject buttons ---

func (b *Bot) onCallback(c telebot.Context) error {
	cb := c.Callback()
	convID, err := uuid.Parse(cb.Data)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "Ошибка данных"})
	}
	ctx := context.Background()
	switch cb.Unique {
	case "take":
		if err := b.owner.Take(ctx, convID); err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: userMessage(err)})
		}
		return c.Respond(&telebot.CallbackResponse{Text: "Взято. Отвечайте reply на сообщения клиента."})
	case "reject":
		if err := b.owner.Reject(ctx, convID); err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: userMessage(err)})
		}
		return c.Respond(&telebot.CallbackResponse{Text: "Отклонено"})
	}
	return nil
}

// --- domain.OwnerNotifier implementation ---

func (b *Bot) NotifyLead(ctx context.Context, ownerChatID int64, n domain.LeadNotification) error {
	var text string
	if n.Fallback {
		text = fmt.Sprintf("[ИИ НЕДОСТУПЕН — ответьте вручную]\nКанал: %s\nКлиент пишет: «%s»\n\nНажмите «Взять» и ответьте клиенту reply на это сообщение.",
			channelName(n.ChannelType), truncate(n.CustomerText, 300))
	} else {
		scoreLine := map[string]string{
			"hot":  "[ГОРЯЧИЙ ЛИД]",
			"warm": "[теплый лид]",
			"cold": "[холодный]",
		}[n.Score]
		if scoreLine == "" {
			scoreLine = "[" + n.Score + "]"
		}
		text = fmt.Sprintf("%s — канал: %s\nЗапрос: %s\nУверенность ИИ: %d%%\n\nКлиент пишет: «%s»\n\nЧерновик ответа ИИ:\n%s",
			scoreLine, channelName(n.ChannelType), firstLine(n.Intent), int(n.Confidence*100), truncate(n.CustomerText, 300), n.DraftReply)
	}
	return b.sendWithButtons(ctx, ownerChatID, n.ConversationID, text)
}

func (b *Bot) RelayCustomerMessage(ctx context.Context, ownerChatID int64, convID uuid.UUID, text, senderName string, withButtons bool) error {
	msg := fmt.Sprintf("Клиент: «%s»", truncate(text, 500))
	if withButtons {
		msg = "Новое сообщение (ИИ уже ответил или ждёт):\n" + msg
	}
	return b.sendWithButtons(ctx, ownerChatID, convID, msg)
}

func (b *Bot) sendWithButtons(ctx context.Context, ownerChatID int64, convID uuid.UUID, text string) error {
	menu := &telebot.ReplyMarkup{}
	menu.Inline(menu.Row(
		telebot.Btn{Unique: "take", Text: "Взять", Data: convID.String()},
		telebot.Btn{Unique: "reject", Text: "Отклонить", Data: convID.String()},
	))
	m, err := b.b.Send(&telebot.Chat{ID: ownerChatID}, text, menu)
	if err != nil {
		return err
	}
	return b.store.SaveOwnerForward(ctx, int64(m.ID), convID)
}

// --- error translation: domain errors -> user-facing text ---

func userMessage(err error) string {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return "Сначала отправьте /start."
	case errors.Is(err, domain.ErrConflict):
		return "Этот бот уже подключён."
	case errors.Is(err, domain.ErrAlreadyHandled):
		return "Уже обработано."
	case errors.Is(err, domain.ErrLeadRejected):
		return "Этот лид отклонён."
	case errors.Is(err, domain.ErrNoForwardMapping):
		return "Не найдено сообщение клиента — ответьте (reply) на его сообщение."
	default:
		return "Ошибка: " + err.Error()
	}
}

const helpText = `Команды:
/connect <токен> — подключить бота-канал (создайте у @BotFather)
/context <описание> — карточка бизнеса для ИИ
/policy — правила автоответов (hot_autoreply, warm_notify, cold_autoreply)
/status — статистика лидов
/help — эта справка

Как это работает:
1. Клиент пишет в подключённого бота.
2. ИИ квалифицирует лид и отвечает первым сообщением.
3. Горячие лиды прилетают сюда с кнопками «Взять / Отклонить».
4. Взяли — отвечайте клиенту reply на его сообщения здесь.`

func channelName(t string) string {
	if t == "telegram" {
		return "Telegram"
	}
	return t
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
