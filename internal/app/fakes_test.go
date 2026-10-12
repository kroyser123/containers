package app

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/google/uuid"

	"ai-inbox/internal/domain"
)

// --- in-memory fakes implementing domain ports ---

// fakeStore implements domain.LeadRepository + domain.TenantRepository.
type fakeStore struct {
	mu            sync.Mutex
	tenantsByChat map[int64]*domain.Tenant
	tenantsByID   map[uuid.UUID]*domain.Tenant
	channels      map[uuid.UUID]*domain.Channel
	convs         map[uuid.UUID]*domain.Conversation
	convByKey     map[string]uuid.UUID // channelID|chatID -> convID
	msgs          map[uuid.UUID][]domain.Message
	msgIDs        map[uuid.UUID]map[string]bool // convID -> external msg ids
	quals         map[uuid.UUID][]domain.QualificationResult
	forwards      map[int64]uuid.UUID
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		tenantsByChat: make(map[int64]*domain.Tenant),
		tenantsByID:   make(map[uuid.UUID]*domain.Tenant),
		channels:      make(map[uuid.UUID]*domain.Channel),
		convs:         make(map[uuid.UUID]*domain.Conversation),
		convByKey:     make(map[string]uuid.UUID),
		msgs:          make(map[uuid.UUID][]domain.Message),
		msgIDs:        make(map[uuid.UUID]map[string]bool),
		quals:         make(map[uuid.UUID][]domain.QualificationResult),
		forwards:      make(map[int64]uuid.UUID),
	}
}

func (s *fakeStore) addTenant(t *domain.Tenant) {
	s.tenantsByChat[t.OwnerChatID] = t
	s.tenantsByID[t.ID] = t
}

func (s *fakeStore) addChannel(ch *domain.Channel) { s.channels[ch.ID] = ch }

func convKey(channelID uuid.UUID, chatID string) string { return channelID.String() + "|" + chatID }

func (s *fakeStore) UpsertConversationAndMessage(_ context.Context, tenantID, channelID uuid.UUID, externalChatID, externalMsgID, text string) (uuid.UUID, domain.ConversationStatus, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := convKey(channelID, externalChatID)
	convID, ok := s.convByKey[key]
	if !ok {
		convID = uuid.New()
		s.convByKey[key] = convID
		s.convs[convID] = &domain.Conversation{
			ID: convID, TenantID: tenantID, ChannelID: channelID,
			ExternalChatID: externalChatID, Status: domain.StatusNew, ChannelType: "telegram",
		}
		s.msgIDs[convID] = make(map[string]bool)
	}
	if s.msgIDs[convID][externalMsgID] {
		return convID, s.convs[convID].Status, false, nil
	}
	s.msgIDs[convID][externalMsgID] = true
	s.msgs[convID] = append(s.msgs[convID], domain.Message{Direction: "in", Text: text})
	return convID, s.convs[convID].Status, true, nil
}

func (s *fakeStore) GetConversation(_ context.Context, id uuid.UUID) (*domain.Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.convs[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *c
	return &cp, nil
}

func (s *fakeStore) GetConversationWithTenant(_ context.Context, id uuid.UUID) (*domain.Conversation, *domain.Tenant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.convs[id]
	if !ok {
		return nil, nil, domain.ErrNotFound
	}
	t, ok := s.tenantsByID[c.TenantID]
	if !ok {
		return nil, nil, domain.ErrNotFound
	}
	cp, tp := *c, *t
	return &cp, &tp, nil
}

func (s *fakeStore) ListLastMessages(_ context.Context, conversationID uuid.UUID, limit int) ([]domain.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msgs := s.msgs[conversationID]
	if len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}
	cp := make([]domain.Message, len(msgs))
	copy(cp, msgs)
	return cp, nil
}

func (s *fakeStore) MarkConversationQualified(_ context.Context, id uuid.UUID, score string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.convs[id]
	if !ok || c.Status != domain.StatusNew {
		return false, nil
	}
	c.Status = domain.StatusAI
	c.Score = score
	return true, nil
}

func (s *fakeStore) TakeConversation(_ context.Context, id uuid.UUID) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.convs[id]
	if !ok || (c.Status != domain.StatusNew && c.Status != domain.StatusAI) {
		return false, nil
	}
	c.Status = domain.StatusHuman
	return true, nil
}

func (s *fakeStore) RejectConversation(_ context.Context, id uuid.UUID) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.convs[id]
	if !ok || (c.Status != domain.StatusNew && c.Status != domain.StatusAI) {
		return false, nil
	}
	c.Status = domain.StatusRejected
	return true, nil
}

func (s *fakeStore) SaveQualification(_ context.Context, conversationID uuid.UUID, res domain.QualificationResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.quals[conversationID] = append(s.quals[conversationID], res)
	return nil
}

func (s *fakeStore) ConversationStats(_ context.Context, tenantID uuid.UUID) (map[string]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]int{}
	for _, c := range s.convs {
		if c.TenantID == tenantID {
			out[string(c.Status)]++
		}
	}
	return out, nil
}

func (s *fakeStore) SaveOwnerForward(_ context.Context, ownerMsgID int64, conversationID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.forwards[ownerMsgID] = conversationID
	return nil
}

func (s *fakeStore) GetConversationByOwnerForward(_ context.Context, ownerMsgID int64) (uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.forwards[ownerMsgID]
	if !ok {
		return uuid.Nil, domain.ErrNotFound
	}
	return id, nil
}

func (s *fakeStore) EnsureTenant(_ context.Context, ownerChatID int64, name string) (*domain.Tenant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.tenantsByChat[ownerChatID]; ok {
		return t, nil
	}
	t := &domain.Tenant{ID: uuid.New(), Name: name, OwnerChatID: ownerChatID, LLMContext: []byte(`{}`), Policy: []byte(`{}`)}
	s.tenantsByChat[ownerChatID] = t
	s.tenantsByID[t.ID] = t
	return t, nil
}

func (s *fakeStore) GetTenantByOwnerChat(_ context.Context, ownerChatID int64) (*domain.Tenant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tenantsByChat[ownerChatID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return t, nil
}

func (s *fakeStore) SetBusinessCard(_ context.Context, _ uuid.UUID, _ string) error { return nil }

func (s *fakeStore) SetPolicyFlag(_ context.Context, tenantID uuid.UUID, key string, on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tenantsByID[tenantID]
	if !ok {
		return domain.ErrNotFound
	}
	p := domain.ParsePolicy(t.Policy)
	switch key {
	case "hot_autoreply":
		p.HotAutoreply = on
	case "warm_notify":
		p.WarmNotify = on
	case "cold_autoreply":
		p.ColdAutoreply = on
	}
	b, _ := json.Marshal(p)
	t.Policy = b
	return nil
}

func (s *fakeStore) CreateChannel(_ context.Context, tenantID uuid.UUID, channelType string, credentials []byte, externalID string) (*domain.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ch := range s.channels {
		if ch.Type == channelType && ch.ExternalID == externalID {
			return nil, domain.ErrConflict
		}
	}
	ch := &domain.Channel{ID: uuid.New(), TenantID: tenantID, Type: channelType, Credentials: credentials, ExternalID: externalID, Status: "active"}
	s.channels[ch.ID] = ch
	return ch, nil
}

func (s *fakeStore) GetChannel(_ context.Context, id uuid.UUID) (*domain.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch, ok := s.channels[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *ch
	return &cp, nil
}

func (s *fakeStore) ListChannels(_ context.Context, channelType, status string) ([]domain.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Channel
	for _, ch := range s.channels {
		if ch.Type == channelType && ch.Status == status {
			out = append(out, *ch)
		}
	}
	return out, nil
}

func (s *fakeStore) SetChannelStatus(_ context.Context, id uuid.UUID, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ch, ok := s.channels[id]; ok {
		ch.Status = status
	}
	return nil
}

// fakeQueue implements domain.TaskQueue.
type fakeQueue struct {
	mu       sync.Mutex
	tasks    []*domain.Task
	nextID   int64
	finished map[int64]bool
	failed   map[int64]bool
}

func newFakeQueue() *fakeQueue {
	return &fakeQueue{finished: make(map[int64]bool), failed: make(map[int64]bool)}
}

func (q *fakeQueue) Enqueue(_ context.Context, kind string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.nextID++
	q.tasks = append(q.tasks, &domain.Task{ID: q.nextID, Kind: kind, Payload: b})
	return nil
}

func (q *fakeQueue) Claim(_ context.Context) (*domain.Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, t := range q.tasks {
		if !q.finished[t.ID] && !q.failed[t.ID] {
			t.Attempts++
			cp := *t
			return &cp, nil
		}
	}
	return nil, nil
}

func (q *fakeQueue) Finish(_ context.Context, id int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.finished[id] = true
	return nil
}

func (q *fakeQueue) Fail(_ context.Context, id int64, _ int) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.failed[id] = true
	return nil
}

func (q *fakeQueue) pendingCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	for _, t := range q.tasks {
		if !q.finished[t.ID] && !q.failed[t.ID] {
			n++
		}
	}
	return n
}

// fakeOutbox implements domain.OutboxRepository.
type fakeOutbox struct {
	mu        sync.Mutex
	items     []*domain.OutboxItem
	nextID    int64
	sent      map[int64]bool
	failed    map[int64]bool
	sentTexts []string
}

func newFakeOutbox() *fakeOutbox {
	return &fakeOutbox{sent: make(map[int64]bool), failed: make(map[int64]bool)}
}

func (o *fakeOutbox) Enqueue(_ context.Context, channelID, conversationID uuid.UUID, text string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.nextID++
	o.items = append(o.items, &domain.OutboxItem{ID: o.nextID, ChannelID: channelID, ConversationID: conversationID, Text: text})
	return nil
}

func (o *fakeOutbox) Claim(_ context.Context) (*domain.OutboxItem, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, it := range o.items {
		if !o.sent[it.ID] && !o.failed[it.ID] {
			it.Attempts++
			cp := *it
			return &cp, nil
		}
	}
	return nil, nil
}

func (o *fakeOutbox) MarkSent(_ context.Context, id int64, _ uuid.UUID, _, text string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sent[id] = true
	o.sentTexts = append(o.sentTexts, text)
	return nil
}

func (o *fakeOutbox) Fail(_ context.Context, id int64, _ int) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.failed[id] = true
	return nil
}

func (o *fakeOutbox) pendingTexts() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	var out []string
	for _, it := range o.items {
		if !o.sent[it.ID] && !o.failed[it.ID] {
			out = append(out, it.Text)
		}
	}
	return out
}

// fakeNotifier implements domain.OwnerNotifier.
type fakeNotifier struct {
	mu     sync.Mutex
	leads  []domain.LeadNotification
	relays []relayRecord
}

type relayRecord struct {
	convID      uuid.UUID
	text        string
	withButtons bool
}

func (n *fakeNotifier) NotifyLead(_ context.Context, _ int64, lead domain.LeadNotification) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.leads = append(n.leads, lead)
	return nil
}

func (n *fakeNotifier) RelayCustomerMessage(_ context.Context, _ int64, convID uuid.UUID, text, _ string, withButtons bool) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.relays = append(n.relays, relayRecord{convID: convID, text: text, withButtons: withButtons})
	return nil
}

// fakeLLM implements domain.LLMQualifier.
type fakeLLM struct {
	mu     sync.Mutex
	result domain.QualificationResult
	err    error
	calls  int
}

func (l *fakeLLM) Qualify(_ context.Context, _, _ string, _ []domain.TranscriptMessage) (domain.QualificationResult, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls++
	return l.result, l.err
}

// fakeSender implements domain.MessageSender.
type fakeSender struct {
	mu    sync.Mutex
	sent  []string
	err   error
}

func (s *fakeSender) Send(_ context.Context, _ uuid.UUID, _, text string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return "", s.err
	}
	s.sent = append(s.sent, text)
	return "1000", nil
}

// fakeMeter implements domain.UsageMeter.
type fakeMeter struct {
	mu      sync.Mutex
	records int
}

func (m *fakeMeter) RecordLLMUsage(_ context.Context, _ uuid.UUID, _ string, _, _ int, _ float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records++
	return nil
}
