package gateway

import (
	"encoding/json"
	"testing"
)

// Volcengine Ark turns its deepseek-v4 models' thinking off with
// thinking.type=disabled, the way its own console sends it: the levels Ark's
// channel borrows from DeepSeek's listing are low, high and max, and
// reasoning_effort is no level it takes for them (pintaste, #1098).
func TestArkDeepseekOffGoesAsThinkingDisabled(t *testing.T) {
	for _, c := range []struct {
		name, host, model, effort string
		wantThinking              bool
	}{
		{"ark's deepseek, off", "ark.cn-beijing.volces.com", "deepseek-v4-flash", "none", true},
		{"ark's deepseek at low", "ark.cn-beijing.volces.com", "deepseek-v4-flash", "low", false},
		{"ark's doubao, off", "ark.cn-beijing.volces.com", "doubao-seed-2", "none", false},
		{"deepseek's own, off", "api.deepseek.com", "deepseek-v4-flash", "none", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := &Request{Effort: c.effort, Messages: []Message{{Role: "user", Parts: []Part{{Kind: Text, Text: "1+1?"}}}}}
			var out map[string]any
			if err := json.Unmarshal(buildChat(r, c.model, c.host, false), &out); err != nil {
				t.Fatal(err)
			}
			th, ok := out["thinking"].(map[string]any)
			if off := ok && th["type"] == "disabled"; off != c.wantThinking {
				t.Fatalf("thinking %v, want off %v (body %v)", out["thinking"], c.wantThinking, out)
			}
			effort, sent := out["reasoning_effort"]
			switch {
			case c.wantThinking && sent:
				t.Errorf("thinking off also sent reasoning_effort %v", effort)
			case !c.wantThinking && effort != c.effort:
				t.Errorf("reasoning_effort %v, want %q", effort, c.effort)
			}
		})
	}
}
