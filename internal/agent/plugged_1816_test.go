package agent

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPluggedEmptyParentsGoWithTheirKeys: the parents magpie's nested keys
// went into are made again when the keys go, so a file the user had comes
// back as it was, byte for byte (#1816). A parent the user had themselves,
// empty or not, stays.
func TestPluggedEmptyParentsGoWithTheirKeys(t *testing.T) {
	home := pluggedHome(t, `
export default {
  agent: { id: "kilo", config: "$HOME/.kilo/kilo.jsonc", model: "model" },
  async connect({ gateway }) {
    return {
      "provider.magpie.options": { baseUrl: gateway.v1, apiKey: gateway.key },
      "extra.magpie.options": { baseUrl: gateway.v1 },
    };
  },
};`)
	path := filepath.Join(home, ".kilo", "kilo.jsonc")
	mine := `{
  // my own provider
  "provider": {
    "openai": { "apiKey": "sk-mine" }
  },
  "extra": {},
  "model": "openai/gpt"
}
`
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := Find("kilo")
	if err != nil {
		t.Fatal(err)
	}
	if a.Plugin == "" || !a.Detected() {
		t.Fatalf("agent: %+v", a)
	}
	f := a.Field("model")
	if err := f.Set("magpie/deepseek/pro"); err != nil {
		t.Fatal(err)
	}
	if err := a.Unwire(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if string(b) != mine {
		t.Fatalf("unwired, not the user's own bytes:\n%s", b)
	}
	// a second wiring takes the same way out
	if err := f.Set("magpie/deepseek/flash"); err != nil {
		t.Fatal(err)
	}
	if err := a.Unwire(); err != nil {
		t.Fatal(err)
	}
	if b, _ = os.ReadFile(path); string(b) != mine {
		t.Fatalf("unwired again, not the user's own bytes:\n%s", b)
	}
}
