package domain

import (
	"encoding/json"
	"testing"
)

func TestApplyPolicyHot(t *testing.T) {
	p := Policy{HotAutoreply: true, WarmNotify: true}
	d := ApplyPolicy(p, ScoreHot, "draft", 0.9)
	if !d.AutoSend || !d.Notify {
		t.Fatalf("hot with autoreply: want AutoSend+Notify, got %+v", d)
	}
}

func TestApplyPolicyHotLowConfidence(t *testing.T) {
	p := Policy{HotAutoreply: true}
	d := ApplyPolicy(p, ScoreHot, "draft", 0.3)
	if d.AutoSend {
		t.Fatalf("hot with low confidence must not auto-send, got %+v", d)
	}
	if !d.Notify {
		t.Fatalf("hot must always notify, got %+v", d)
	}
}

func TestApplyPolicyHotAutoreplyOff(t *testing.T) {
	p := Policy{HotAutoreply: false}
	d := ApplyPolicy(p, ScoreHot, "draft", 0.9)
	if d.AutoSend {
		t.Fatalf("hot with autoreply off must not auto-send, got %+v", d)
	}
}

func TestApplyPolicyWarm(t *testing.T) {
	p := Policy{WarmNotify: true}
	d := ApplyPolicy(p, ScoreWarm, "draft", 0.9)
	if d.AutoSend || !d.Notify {
		t.Fatalf("warm: want notify only, got %+v", d)
	}
}

func TestApplyPolicyCold(t *testing.T) {
	p := Policy{ColdAutoreply: false}
	d := ApplyPolicy(p, ScoreCold, "draft", 0.9)
	if d.AutoSend || d.Notify {
		t.Fatalf("cold without autoreply: want nothing, got %+v", d)
	}
	p.ColdAutoreply = true
	d = ApplyPolicy(p, ScoreCold, "draft", 0.9)
	if !d.AutoSend || d.Notify {
		t.Fatalf("cold with autoreply: want auto-send only, got %+v", d)
	}
}

func TestParsePolicyDefaults(t *testing.T) {
	p := ParsePolicy(nil)
	if !p.HotAutoreply || !p.WarmNotify || p.ColdAutoreply {
		t.Fatalf("defaults wrong: %+v", p)
	}
}

func TestParsePolicyPartialOverride(t *testing.T) {
	p := ParsePolicy(json.RawMessage(`{"hot_autoreply":false}`))
	if p.HotAutoreply {
		t.Fatalf("override not applied: %+v", p)
	}
	if !p.WarmNotify {
		t.Fatalf("default lost: %+v", p)
	}
}
