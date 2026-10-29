package gateway

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

// pngBase64 is a w by h PNG's base64, as an agent sends a screenshot.
func pngBase64(t *testing.T, w, h int) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// An image part costs what its size costs, not nothing: estimate() read a
// part's text and byte length, and an image has neither, so a request
// carrying screenshots counted as its text alone — routing rules and the
// count_tokens endpoints read exactly that number, and a session with ~55
// full-size screenshots was taken for 698,000 tokens where the vendor
// counted 1,000,000, so nothing compacted and every request came back
// "Prompt is too long" (#4239).
func TestEstimateCountsImages(t *testing.T) {
	text := Request{Messages: []Message{{Role: "user", Parts: []Part{{Kind: Text, Text: "How many images?"}}}}}
	for _, c := range []struct{ w, h, want int }{
		{1024, 2048, 5242}, // the vendor counted 5,228
		{512, 1024, 2500},  // 2,469
		{256, 512, 2500},   // 2,788
	} {
		req := Request{Messages: []Message{{Role: "user", Parts: []Part{
			{Kind: Text, Text: "How many images?"},
			{Kind: Image, MediaType: "image/png", Data: pngBase64(t, c.w, c.h)},
		}}}}
		if got, base := estimate(&req)-estimate(&text), c.want; got != base {
			t.Errorf("%dx%d: an image added %d tokens to estimate(), want %d", c.w, c.h, got, base)
		}
	}
	// an image whose bytes can't be read is still no image of no cost
	req := Request{Messages: []Message{{Role: "user", Parts: []Part{{Kind: Image, Data: "AAAA"}}}}}
	if got := estimate(&req); got != imageTokens {
		t.Errorf("an unreadable image counted %d, want %d", got, imageTokens)
	}
}

// The prompt's parts count an image the same way, from the size its source
// JSON carries, whichever protocol wrote it: Anthropic's source object,
// Chat's image_url and Responses' input_image.
func TestPromptPartsCountImageSize(t *testing.T) {
	png := pngBase64(t, 1024, 2048)
	for _, c := range []struct {
		proto provider.Protocol
		body  string
	}{
		{provider.Anthropic, `{"model":"m","messages":[{"role":"user","content":[{"type":"text","text":"how many"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"` + png + `"}}]}]}`},
		{provider.Chat, `{"model":"m","messages":[{"role":"user","content":[{"type":"text","text":"how many"},{"type":"image_url","image_url":{"url":"data:image/png;base64,` + png + `"}}]}]}`},
		{provider.Responses, `{"model":"m","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"how many"},{"type":"input_image","image_url":"data:image/png;base64,` + png + `"}]}]}`},
	} {
		p := promptOf(c.proto, []byte(c.body))
		if p == nil {
			t.Fatalf("%s: no prompt", c.proto)
		}
		it, ok := itemOf(p, PartFiles, "image")
		if !ok || it.Tokens != imageCost(1024, 2048) {
			t.Errorf("%s: image %+v, want %d tokens", c.proto, it, imageCost(1024, 2048))
		}
	}
}

// A source whose base64 can't be read, and one that carries no image at
// all, keep the flat imageTokens: a vendor that does read the image still
// costs something for it.
func TestPromptImageWithoutAHeaderKeepsTheFlatCost(t *testing.T) {
	for _, c := range []struct {
		proto provider.Protocol
		body  string
	}{
		{provider.Anthropic, `{"model":"m","messages":[{"role":"user","content":[{"type":"image","source":{"type":"url","url":"https://example.com/a.png"}}]}]}`},
		{provider.Chat, `{"model":"m","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}]}]}`},
	} {
		p := promptOf(c.proto, []byte(c.body))
		if p == nil {
			t.Fatalf("%s: no prompt", c.proto)
		}
		if it, ok := itemOf(p, PartFiles, "image"); !ok || it.Tokens != imageTokens {
			t.Errorf("%s: image %+v, want %d tokens", c.proto, it, imageTokens)
		}
	}
}

// The base64 is read where it is, not copied out of the source: a source
// holding megabytes of it is a subslice and a header's read.
func TestImageBase64IsASubslice(t *testing.T) {
	png := pngBase64(t, 4, 4)
	src := []byte(`{"type":"base64","media_type":"image/png","data":"` + png + `"}`)
	got := imageBase64(src)
	if string(got) != png {
		t.Fatalf("base64 %d bytes, want %d", len(got), len(png))
	}
	if &got[0] != &src[len(src)-len(png)-2] {
		t.Error("the base64 was copied, not read where it is")
	}
	// an escape is not base64: nothing is read from it
	var b strings.Builder
	json.NewEncoder(&b).Encode(map[string]string{"data": `has "a quote`})
	if got := imageBase64([]byte(b.String())); got != nil {
		t.Errorf("an escaped source read %q", got)
	}
}
