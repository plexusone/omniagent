package meeting

import (
	"strings"
	"testing"
	"time"

	"github.com/plexusone/omnivoice-core/audio/format"
	"github.com/plexusone/omnivoice-core/tts"
)

func TestParseSummary(t *testing.T) {
	for _, reply := range []string{
		`{"overview": "ok", "decisions": ["a"]}`,
		"```json\n{\"overview\": \"ok\", \"decisions\": [\"a\"]}\n```",
		"Here you go:\n{\"overview\": \"ok\", \"decisions\": [\"a\"]}",
	} {
		s, err := parseSummary(reply)
		if err != nil {
			t.Fatalf("parseSummary(%q) error: %v", reply, err)
		}
		if s.Overview != "ok" || len(s.Decisions) != 1 {
			t.Fatalf("parseSummary(%q) = %+v", reply, s)
		}
	}
	for _, reply := range []string{"no json here", `{"overview": `} {
		if _, err := parseSummary(reply); err == nil {
			t.Errorf("parseSummary(%q) = nil error", reply)
		}
	}
}

func TestSummaryMarkdown(t *testing.T) {
	md := Summary{
		Overview:      "Set the launch date.",
		Decisions:     []string{"Launch Thursday"},
		ActionItems:   []ActionItem{{Task: "Update roadmap", Owner: "Dana", Due: "Friday"}, {Task: "Book room"}},
		OpenQuestions: []string{"Budget?"},
	}.Markdown()
	for _, want := range []string{
		"## Summary\n\nSet the launch date.",
		"## Decisions\n\n- Launch Thursday",
		"- Update roadmap (owner: Dana, due: Friday)",
		"- Book room\n",
		"## Open Questions\n\n- Budget?",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("Markdown() missing %q:\n%s", want, md)
		}
	}
	if strings.Contains(Summary{Overview: "x"}.Markdown(), "## Decisions") {
		t.Error("Markdown() lists empty sections")
	}
}

func TestTranscriptString(t *testing.T) {
	got := Transcript{
		{Start: 65 * time.Second, Text: "First."},
		{Start: time.Hour + 2*time.Second, Speaker: "Dana", Text: "Second."},
	}.String()
	want := "[00:01:05] First.\n[01:00:02] Dana: Second.\n"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestSynthesizedFormat(t *testing.T) {
	tests := []struct {
		format string
		rate   int
		want   format.AudioFormat
		ok     bool
	}{
		{"pcm", 24000, format.AudioFormat{Encoding: "linear16", SampleRate: 24000, Channels: 1}, true},
		{"pcm_16000", 0, format.AudioFormat{Encoding: "linear16", SampleRate: 16000, Channels: 1}, true},
		{"ulaw_8000", 0, format.AudioFormat{Encoding: "mulaw", SampleRate: 8000, Channels: 1}, true},
		{"mulaw", 0, format.AudioFormat{Encoding: "mulaw", SampleRate: 8000, Channels: 1}, true},
		{"mp3", 44100, format.AudioFormat{}, false},
		{"pcm", 0, format.AudioFormat{}, false},
	}
	for _, tt := range tests {
		got, err := synthesizedFormat(&tts.SynthesisResult{Format: tt.format, SampleRate: tt.rate})
		if (err == nil) != tt.ok || got != tt.want {
			t.Errorf("synthesizedFormat(%q, %d) = %+v, %v", tt.format, tt.rate, got, err)
		}
	}
}

func TestDefaultAnnouncement(t *testing.T) {
	got := DefaultAnnouncement("OmniAgent", "Jane")
	for _, want := range []string{"OmniAgent", "AI assistant", "attending for Jane", "transcribing"} {
		if !strings.Contains(got, want) {
			t.Errorf("DefaultAnnouncement() = %q, missing %q", got, want)
		}
	}
	if strings.Contains(DefaultAnnouncement("OmniAgent", ""), "attending for") {
		t.Error("announcement names someone when OnBehalfOf is empty")
	}
}
