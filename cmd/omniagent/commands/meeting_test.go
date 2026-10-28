package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/plexusone/omnivoice-core/stt"

	"github.com/plexusone/omnimeet-core/attendee"
	"github.com/plexusone/omnimeet-core/dialin"
	"github.com/plexusone/omnimeet-core/dialin/dialintest"

	"github.com/plexusone/omniagent/meeting"
)

const zoomInvite = `Join Zoom Meeting
https://us02web.zoom.us/j/85012345678?pwd=AbCdEf

Meeting ID: 850 1234 5678
Passcode: 482913

One tap mobile
+16695550100,,85012345678#,,,,*482913# US (San Jose)
`

const zoomICS = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nSUMMARY:Review\r\n" +
	"DESCRIPTION:Join Zoom Meeting\\nhttps://us02web.zoom.us/j/85012345678\\nMeeting ID: 850 1234 5678\\n" +
	"Passcode: 482913\\n+16695550100\\,\\,85012345678#\\,\\,\\,\\,*482913# US (San Jose)\r\n" +
	"END:VEVENT\r\nEND:VCALENDAR\r\n"

// lineTranscriber treats each audio chunk as the words spoken.
type lineTranscriber struct{}

func (lineTranscriber) TranscribeStream(context.Context, stt.TranscriptionConfig) (io.WriteCloser, <-chan stt.StreamEvent, error) {
	events := make(chan stt.StreamEvent, 16)
	return &lineStream{events: events}, events, nil
}

type lineStream struct {
	events chan stt.StreamEvent
	once   sync.Once
}

func (s *lineStream) Write(p []byte) (int, error) {
	s.events <- stt.StreamEvent{Type: stt.EventTranscript, Transcript: string(p), IsFinal: true}
	return len(p), nil
}

func (s *lineStream) Close() error {
	s.once.Do(func() { close(s.events) })
	return nil
}

type fixedSummarizer struct{}

func (fixedSummarizer) Summarize(context.Context, meeting.Transcript) (*meeting.Summary, error) {
	return &meeting.Summary{Overview: "Launch moved.", Decisions: []string{"Launch Thursday"}}, nil
}

// syncBuffer is a bytes.Buffer safe for the transcript callback.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestAttendMeetingFromInvite(t *testing.T) {
	target, err := readMeetingInvite("-", strings.NewReader(zoomInvite))
	if err != nil {
		t.Fatalf("readMeetingInvite() error: %v", err)
	}
	sim := &dialintest.Meeting{
		Platform: attendee.PlatformZoom, Number: "+16695550100",
		ConferenceID: "85012345678", Passcode: "482913",
		Script: [][]byte{[]byte("Let's launch Thursday.")}, EndAfterScript: true,
	}
	out := &syncBuffer{}

	rec, err := attendMeeting(context.Background(), meetingDeps{
		Gateway:     dialintest.NewGateway(sim),
		Transcriber: lineTranscriber{},
		Summarizer:  fixedSummarizer{},
		JoinOptions: []dialin.JoinerOption{
			dialin.WithTiming(dialin.Timing{Answer: time.Millisecond, Between: time.Millisecond}),
			dialin.WithKeyDuration(0),
		},
	}, target, meetingJoinOptions{Country: "US", Name: "OmniAgent"}, out)
	if err != nil {
		t.Fatalf("attendMeeting() error: %v", err)
	}

	if ok, reason := sim.Calls()[0].Admitted(); !ok {
		t.Fatalf("simulated meeting rejected the keypad sequence: %s", reason)
	}
	if !strings.Contains(out.String(), "] Let's launch Thursday.") {
		t.Errorf("live transcript output = %q", out.String())
	}
	if rec.FinalState != attendee.StateEnded || rec.Summary == nil || rec.Summary.Decisions[0] != "Launch Thursday" {
		t.Errorf("record = %+v", rec)
	}
}

func TestAttendMeetingWithoutDialIn(t *testing.T) {
	target, err := readMeetingInvite("-", strings.NewReader("Join: https://meet.google.com/abc-defg-hij"))
	if err != nil {
		t.Fatalf("readMeetingInvite() error: %v", err)
	}
	_, err = attendMeeting(context.Background(), meetingDeps{Gateway: dialintest.NewGateway(), Transcriber: lineTranscriber{}},
		target, meetingJoinOptions{Country: "US"}, io.Discard)
	if !errors.Is(err, dialin.ErrNoDialIn) {
		t.Fatalf("attendMeeting() error = %v, want ErrNoDialIn", err)
	}
}

func TestReadMeetingInvite(t *testing.T) {
	dir := t.TempDir()
	textPath := filepath.Join(dir, "invite.txt")
	icsPath := filepath.Join(dir, "invite.ics")
	for path, body := range map[string]string{textPath: zoomInvite, icsPath: zoomICS} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	for name, read := range map[string]func() (attendee.JoinTarget, error){
		"text file":  func() (attendee.JoinTarget, error) { return readMeetingInvite(textPath, nil) },
		"ics file":   func() (attendee.JoinTarget, error) { return readMeetingInvite(icsPath, nil) },
		"stdin ics":  func() (attendee.JoinTarget, error) { return readMeetingInvite("-", strings.NewReader(zoomICS)) },
		"stdin text": func() (attendee.JoinTarget, error) { return readMeetingInvite("-", strings.NewReader(zoomInvite)) },
	} {
		t.Run(name, func(t *testing.T) {
			target, err := read()
			if err != nil {
				t.Fatalf("readMeetingInvite() error: %v", err)
			}
			if target.Platform != attendee.PlatformZoom || target.DialIn == nil ||
				target.DialIn.ConferenceID != "85012345678" || target.DialIn.Passcode != "482913" {
				t.Fatalf("target = %+v", target)
			}
		})
	}

	if _, err := readMeetingInvite(filepath.Join(dir, "missing.ics"), nil); err == nil {
		t.Error("readMeetingInvite() of a missing file returned nil error")
	}
}

func TestWriteMeetingRecord(t *testing.T) {
	dir := t.TempDir()
	rec := &meeting.Record{
		Target:     attendee.JoinTarget{Platform: attendee.PlatformZoom, JoinURL: "https://us02web.zoom.us/j/1"},
		Started:    time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC),
		Ended:      time.Date(2026, 10, 28, 9, 30, 0, 0, time.UTC),
		FinalState: attendee.StateLeft,
		Transcript: meeting.Transcript{{Start: 5 * time.Second, Text: "Hello."}},
		Summary:    &meeting.Summary{Overview: "Quick sync."},
	}
	o := meetingJoinOptions{Out: filepath.Join(dir, "notes.md"), JSONOut: filepath.Join(dir, "record.json")}
	var out bytes.Buffer

	if err := writeMeetingRecord(rec, o, &out); err != nil {
		t.Fatalf("writeMeetingRecord() error: %v", err)
	}
	notes, err := os.ReadFile(o.Out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Meeting Notes", "**Duration:** 30m0s", "**Ended as:** left", "Quick sync.", "[00:00:05] Hello."} {
		if !strings.Contains(string(notes), want) {
			t.Errorf("notes missing %q:\n%s", want, notes)
		}
	}
	var decoded meeting.Record
	data, err := os.ReadFile(o.JSONOut)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &decoded); err != nil || decoded.Summary.Overview != "Quick sync." {
		t.Errorf("JSON record = %s, err %v", data, err)
	}
	if !strings.Contains(out.String(), "Meeting notes written to "+o.Out) {
		t.Errorf("output = %q", out.String())
	}
}

func TestProviderAPIKey(t *testing.T) {
	t.Setenv("DEEPGRAM_API_KEY", "dg-key")
	t.Setenv("NEWVOICE_API_KEY", "nv-key")
	t.Setenv("ELEVENLABS_API_KEY", "")
	if key, err := providerAPIKey("deepgram"); err != nil || key != "dg-key" {
		t.Errorf("deepgram key = %q, %v", key, err)
	}
	if key, err := providerAPIKey("newvoice"); err != nil || key != "nv-key" {
		t.Errorf("unlisted provider key = %q, %v", key, err)
	}
	if _, err := providerAPIKey("elevenlabs"); err == nil || !strings.Contains(err.Error(), "ELEVENLABS_API_KEY") {
		t.Errorf("missing key error = %v", err)
	}
}

func TestSynthesisConfigAndModels(t *testing.T) {
	for _, provider := range []string{"deepgram", "elevenlabs"} {
		if cfg := synthesisConfig(provider, "v1"); cfg.OutputFormat != "" || cfg.VoiceID != "v1" {
			t.Errorf("%s config = %+v, want the call's own format", provider, cfg)
		}
	}
	if cfg := synthesisConfig("openai", ""); cfg.OutputFormat != "pcm" || cfg.SampleRate != 24000 {
		t.Errorf("openai config = %+v", cfg)
	}
	for llm, want := range map[string]string{"anthropic": "claude-sonnet-5", "openai": "gpt-4o", "gemini": "gemini-2.5-flash"} {
		if got := defaultSummaryModel(llm); got != want {
			t.Errorf("defaultSummaryModel(%q) = %q, want %q", llm, got, want)
		}
	}
}

func TestRunMeetingJoinRequiresPublicURL(t *testing.T) {
	saved := meetingJoinOpts
	t.Cleanup(func() { meetingJoinOpts = saved })
	meetingJoinOpts = meetingJoinOptions{Invite: "-"}

	err := runMeetingJoin(meetingJoinCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "--public-url") {
		t.Fatalf("runMeetingJoin() error = %v, want a --public-url error", err)
	}
}

func TestNewMeetingLLM(t *testing.T) {
	for _, env := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY"} {
		t.Setenv(env, "")
	}
	if _, _, err := newMeetingLLM(meetingJoinOptions{}); err == nil || !strings.Contains(err.Error(), "no provider") {
		t.Errorf("without API keys: error = %v", err)
	}

	t.Setenv("OPENAI_API_KEY", "test-api-key")
	client, model, err := newMeetingLLM(meetingJoinOptions{})
	if err != nil {
		t.Fatalf("newMeetingLLM() error: %v", err)
	}
	if client == nil || model != "gpt-4o" {
		t.Errorf("detected LLM = %v, %q; want an OpenAI client with gpt-4o", client, model)
	}
	if _, model, err := newMeetingLLM(meetingJoinOptions{LLM: "openai", Model: "gpt-5"}); err != nil || model != "gpt-5" {
		t.Errorf("explicit model = %q, %v", model, err)
	}
}

func TestMeetingJoinAnswerFlags(t *testing.T) {
	f := meetingJoinCmd.Flags()
	for _, name := range []string{"no-answer", "answer-to"} {
		if f.Lookup(name) == nil {
			t.Errorf("meeting join has no --%s flag", name)
		}
	}
}
