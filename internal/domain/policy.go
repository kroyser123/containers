package domain

import "encoding/json"

// Policy is the per-tenant automation rule set.
type Policy struct {
	HotAutoreply  bool `json:"hot_autoreply"`  // auto-send draft to hot leads
	WarmNotify    bool `json:"warm_notify"`    // notify owner with draft for warm leads
	ColdAutoreply bool `json:"cold_autoreply"` // auto-reply to cold leads
}

func DefaultPolicy() Policy {
	return Policy{HotAutoreply: true, WarmNotify: true, ColdAutoreply: false}
}

// ParsePolicy decodes tenant policy JSONB, falling back to defaults for unset fields.
func ParsePolicy(raw json.RawMessage) Policy {
	p := DefaultPolicy()
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &p)
	}
	return p
}

// Decision is what the policy engine wants to happen for a qualified lead.
type Decision struct {
	AutoSend bool   // put draft into outbox
	Draft    string // text to auto-send (if AutoSend)
	Notify   bool   // push to owner with take/reject buttons
}

const MinConfidence = 0.5

func ApplyPolicy(p Policy, score Score, draft string, confidence float64) Decision {
	switch score {
	case ScoreHot:
		return Decision{
			AutoSend: p.HotAutoreply && confidence >= MinConfidence,
			Draft:    draft,
			Notify:   true,
		}
	case ScoreWarm:
		return Decision{
			AutoSend: false,
			Draft:    draft,
			Notify:   p.WarmNotify,
		}
	default: // cold
		return Decision{
			AutoSend: p.ColdAutoreply,
			Draft:    draft,
			Notify:   false,
		}
	}
}
