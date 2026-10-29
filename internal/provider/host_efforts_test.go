package provider

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/catalog"
)

// ARNO on Discord: 我发现是之前很多能自动推导 reasoning_efforts 列表的模型
// 现在都推导失败了. The Azure OpenAI preset (Catalog "azure, openai") was
// taken for a maker, so models.dev's azure list — Kimi K2.5, DeepSeek V3.2
// and R1, Grok 4.20 Reasoning, served there with no levels listed — was the
// maker's word on them: a relay serving kimi-k2.5 or deepseek-v3.2 took no
// levels, though the providers listing any give them, and the agents
// offered no thinking for it. Azure hosts other makers' models, as Groq
// does; its own provider still takes its own list's word.
func TestHostListIsNoMakersWord(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	if err := os.MkdirAll(filepath.Dir(catalog.CachePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalog.CachePath(), []byte(`{
	  "azure": {"models": {
	    "kimi-k2.5": {"id":"kimi-k2.5","reasoning":true},
	    "deepseek-v3.2": {"id":"deepseek-v3.2","reasoning":true}}},
	  "novita": {"models": {
	    "kimi-k2.5": {"id":"kimi-k2.5","reasoning":true,"reasoning_options":[{"type":"effort","values":["low","medium","high"]}]},
	    "deepseek-v3.2": {"id":"deepseek-v3.2","reasoning":true,"reasoning_options":[{"type":"effort","values":["none","high"]}]}}},
	  "deepinfra": {"models": {
	    "kimi-k2.5": {"id":"kimi-k2.5","reasoning":true,"reasoning_options":[{"type":"effort","values":["low","medium","high"]}]},
	    "deepseek-v3.2": {"id":"deepseek-v3.2","reasoning":true,"reasoning_options":[{"type":"effort","values":["none","high"]}]}}}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog.Reset()
	t.Cleanup(catalog.Reset)

	if slices.Contains(makerCatalogs(), "azure") {
		t.Errorf("azure is taken for a maker: %v", makerCatalogs())
	}
	if err := Save(Provider{ID: "relay", Name: "Relay", Chat: "https://relay.test/v1", Key: "k", Models: []string{"kimi-k2.5", "deepseek-v3.2"}}); err != nil {
		t.Fatal(err)
	}
	if err := catalog.SaveLive("relay", "https://relay.test/v1", []catalog.Model{{ID: "kimi-k2.5"}, {ID: "deepseek-v3.2"}}); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, e := range Catalog() {
		got[e.ID] = strings.Join(e.Efforts, ",")
	}
	for id, want := range map[string]string{
		"relay/kimi-k2.5":     "low,medium,high",
		"relay/deepseek-v3.2": "none,high",
	} {
		if got[id] != want {
			t.Errorf("%s: levels %q, want %q", id, got[id], want)
		}
	}
	relay, err := Find("relay")
	if err != nil {
		t.Fatal(err)
	}
	if relay.Levelless("kimi-k2.5") {
		t.Error("relay's kimi-k2.5 is said to have no levels")
	}
}

// A provider borrowing DeepSeek's levels takes none of its own: what
// accepts "none" is DeepSeek's API, not a reseller's. Volcengine Ark's is
// verified to turn its deepseek-v4 models' thinking off as well, with
// thinking.type=disabled, so its entries say the off level and a relay's
// do not (pintaste, #1098).
func TestArkDeepseekOffersThinkingOff(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	body, err := os.ReadFile(filepath.Join("..", "catalog", "testdata", "models.dev-2026-10-09.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(catalog.CachePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalog.CachePath(), body, 0o644); err != nil {
		t.Fatal(err)
	}
	catalog.Reset()
	t.Cleanup(catalog.Reset)

	for _, tc := range []struct {
		p    Provider
		want string
	}{
		{Provider{ID: "volcengine", Name: "Ark", Chat: "https://ark.cn-beijing.volces.com/api/v3", Key: "k", Models: []string{"deepseek-v4-pro"}}, "none,low,high,max"},
		{Provider{ID: "relay", Name: "Relay", Chat: "https://relay.test/v1", Key: "k", Models: []string{"deepseek-v4-pro"}}, "low,high,max"},
	} {
		if err := Save(tc.p); err != nil {
			t.Fatal(err)
		}
		if err := catalog.SaveLive(tc.p.ID, tc.p.Chat, []catalog.Model{{ID: "deepseek-v4-pro"}}); err != nil {
			t.Fatal(err)
		}
		got, err := Find(tc.p.ID)
		if err != nil {
			t.Fatal(err)
		}
		if lv := strings.Join(got.Efforts("deepseek-v4-pro"), ","); lv != tc.want {
			t.Errorf("%s: levels %q, want %q", tc.p.ID, lv, tc.want)
		}
	}
}

func TestBorrowedOffLevelIsNotAdvertised(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	body, err := os.ReadFile(filepath.Join("..", "catalog", "testdata", "models.dev-2026-10-09.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(catalog.CachePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalog.CachePath(), body, 0o644); err != nil {
		t.Fatal(err)
	}
	catalog.Reset()
	t.Cleanup(catalog.Reset)

	for _, p := range []Provider{
		{ID: "relay", Name: "Relay", Chat: "https://relay.test/v1", Key: "k", Models: []string{"deepseek-v4-pro"}},
		{ID: "th", Name: "TokenHub", Chat: "https://tokenhub.test/v1", Key: "k", Catalog: "tencent-tokenhub", Models: []string{"deepseek-v4-pro"}},
	} {
		if err := Save(p); err != nil {
			t.Fatal(err)
		}
		if err := catalog.SaveLive(p.ID, p.Chat, []catalog.Model{{ID: "deepseek-v4-pro"}}); err != nil {
			t.Fatal(err)
		}
		got, err := Find(p.ID)
		if err != nil {
			t.Fatal(err)
		}
		if efforts := got.Efforts("deepseek-v4-pro"); !slices.Equal(efforts, []string{"low", "high", "max"}) {
			t.Errorf("%s borrowed efforts = %v, want [low high max]", p.ID, efforts)
		}
	}

}
