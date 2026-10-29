package meeting

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/plexusone/omnivoice-core/audio/format"
	"github.com/plexusone/omnivoice-core/tts"

	"github.com/plexusone/omnimeet-core/attendee"
)

func TestAddressed(t *testing.T) {
	names := []string{"OmniAgent", "Omni"}
	tests := []struct {
		text         string
		ok, nameOnly bool
	}{
		{"OmniAgent, what did we decide?", true, false},
		{"what did we decide, omniagent?", true, false},
		{"Omni Agent, can you recap?", true, false},
		{"omni-agent can you recap", true, false},
		{"Hey OmniAgent?", true, true},
		{"Okay, Omni Agent.", true, true},
		{"Omni, recap please.", true, false},
		{"Let's ship on Friday.", false, false},
		{"The omnibus bill passed.", false, false},
		{"Omniagents are everywhere.", false, false},
		{"", false, false},
	}
	for _, tt := range tests {
		ok, nameOnly := addressed(tt.text, names)
		if ok != tt.ok || nameOnly != tt.nameOnly {
			t.Errorf("addressed(%q) = %v, %v; want %v, %v", tt.text, ok, nameOnly, tt.ok, tt.nameOnly)
		}
	}
}

func TestLLMResponderSendsNoToolsAndMarksSpeechUntrusted(t *testing.T) {
	llm := &jsonLLM{reply: "  We decided to launch on Thursday.  "}
	answer, err := LLMResponder{Client: llm, Model: "test-model"}.Respond(context.Background(), Request{
		Name:       "OmniAgent",
		Question:   "OmniAgent, what did we decide?",
		Transcript: Transcript{{Text: "Let's launch on Thursday."}, {Text: "OmniAgent, what did we decide?"}},
	})
	if err != nil {
		t.Fatalf("Respond() error: %v", err)
	}
	if answer != "We decided to launch on Thursday." {
		t.Errorf("answer = %q", answer)
	}
	req := llm.request
	if len(req.Tools) != 0 || req.ToolChoice != nil {
		t.Errorf("request offers tools: %+v", req.Tools)
	}
	system, user := req.Messages[0].Content, req.Messages[1].Content
	for _, want := range []string{"You are OmniAgent", "untrusted", "no tools"} {
		if !strings.Contains(system, want) {
			t.Errorf("system prompt missing %q", want)
		}
	}
	if !strings.Contains(user, "Let's launch on Thursday.") || !strings.Contains(user, "what did we decide?") {
		t.Errorf("user message = %q", user)
	}
	if _, err := (LLMResponder{}).Respond(context.Background(), Request{}); err == nil {
		t.Error("Respond() without a client returned nil error")
	}
}

// liveSession is a meeting session the test drives: it admits the agent,
// plays lines of speech, and records the audio the agent sends.
type liveSession struct {
	events chan attendee.Event
	audioR *io.PipeReader
	audioW *io.PipeWriter

	mu    sync.Mutex
	sent  int
	state attendee.State
	once  sync.Once
}

func newLiveSession() *liveSession {
	r, w := io.Pipe()
	return &liveSession{events: make(chan attendee.Event, 4), audioR: r, audioW: w, state: attendee.StateConnected}
}

func (s *liveSession) ID() string                  { return "live" }
func (s *liveSession) Target() attendee.JoinTarget { return attendee.JoinTarget{} }
func (s *liveSession) Capabilities() attendee.Capabilities {
	return attendee.Capabilities{ReceiveAudio: true, SendAudio: true}
}
func (s *liveSession) State() attendee.State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}
func (s *liveSession) Events() <-chan attendee.Event { return s.events }
func (s *liveSession) Leave(context.Context) error {
	s.once.Do(func() {
		s.mu.Lock()
		s.state = attendee.StateLeft
		s.mu.Unlock()
		close(s.events)
	})
	return s.audioW.Close()
}
func (s *liveSession) ReceiveFormat() format.AudioFormat { return format.Twilio }
func (s *liveSession) ReceiveAudio() io.Reader           { return s.audioR }
func (s *liveSession) SendFormat() format.AudioFormat    { return format.Twilio }
func (s *liveSession) SendAudio() io.Writer              { return sentCounter{s} }

type sentCounter struct{ s *liveSession }

func (c sentCounter) Write(p []byte) (int, error) {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	c.s.sent++
	return len(p), nil
}

func (s *liveSession) clipsSent() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sent
}

func (s *liveSession) admit() {
	s.events <- attendee.Event{Type: attendee.EventStateChanged, SessionID: "live", State: attendee.StateAdmitted}
}

// say plays one line of speech, which textTranscriber turns into a segment.
func (s *liveSession) say(t *testing.T, line string) {
	t.Helper()
	if _, err := s.audioW.Write([]byte(line)); err != nil {
		t.Fatalf("say %q: %v", line, err)
	}
}

// tinySpeaker returns an empty clip, which plays for no time, so the agent
// listens again as soon as it has spoken. A clip with any length would leave
// a window in which a line said right after it is ignored as possible echo.
type tinySpeaker struct {
	mu      sync.Mutex
	texts   []string
	configs []tts.SynthesisConfig
}

func (p *tinySpeaker) Name() string { return "tiny" }
func (p *tinySpeaker) Synthesize(_ context.Context, text string, config tts.SynthesisConfig) (*tts.SynthesisResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.texts = append(p.texts, text)
	p.configs = append(p.configs, config)
	return &tts.SynthesisResult{Format: "ulaw_8000"}, nil
}
func (p *tinySpeaker) SynthesizeStream(context.Context, string, tts.SynthesisConfig) (<-chan tts.StreamChunk, error) {
	return nil, errors.New("not supported")
}
func (p *tinySpeaker) ListVoices(context.Context) ([]tts.Voice, error)      { return nil, nil }
func (p *tinySpeaker) GetVoice(context.Context, string) (*tts.Voice, error) { return nil, nil }

func (p *tinySpeaker) said() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.texts...)
}

// recordingResponder answers every question with a fixed reply.
type recordingResponder struct {
	reply string
	mu    sync.Mutex
	reqs  []Request
}

func (r *recordingResponder) Respond(_ context.Context, req Request) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reqs = append(r.reqs, req)
	return r.reply, nil
}

func (r *recordingResponder) requests() []Request {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Request(nil), r.reqs...)
}

// waitFor polls cond until it holds or the test times out.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// attendLive runs an Attendant on a live session in the background.
func attendLive(t *testing.T, cfg Config) (*liveSession, *segmentLog, func() *Record) {
	t.Helper()
	sess := newLiveSession()
	segs := &segmentLog{}
	cfg.Transcriber = &textTranscriber{}
	cfg.OnSegment = segs.add
	a := New(cfg)
	a.echoGuard = 0

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan *Record, 1)
	go func() {
		rec, err := a.Attend(ctx, sess)
		if err != nil {
			t.Errorf("Attend() error: %v", err)
		}
		done <- rec
	}()
	return sess, segs, func() *Record {
		cancel()
		return <-done
	}
}

type segmentLog struct {
	mu   sync.Mutex
	segs []Segment
}

func (l *segmentLog) add(s Segment) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.segs = append(l.segs, s)
}

func (l *segmentLog) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.segs)
}

func TestAttendAnswersWhenAddressed(t *testing.T) {
	speaker := &tinySpeaker{}
	responder := &recordingResponder{reply: "We decided to ship on Friday."}
	sess, segs, finish := attendLive(t, Config{Name: "OmniAgent", Speaker: speaker, Responder: responder})

	sess.admit()
	waitFor(t, "the announcement", func() bool { return sess.clipsSent() == 1 })
	sess.say(t, "Let's ship on Friday.")
	sess.say(t, "OmniAgent, what did we decide?")
	waitFor(t, "the answer", func() bool { return sess.clipsSent() == 2 && segs.count() == 3 })
	rec := finish()

	said := speaker.said()
	if !strings.HasSuffix(said[0], answerInvitation) {
		t.Errorf("announcement %q does not invite questions", said[0])
	}
	if said[1] != "We decided to ship on Friday." {
		t.Errorf("spoken answer = %q", said[1])
	}
	// With no output format set, speech is requested in the call's format,
	// so any provider that produces mu-law at 8 kHz plays without converting.
	if cfg := speaker.configs[0]; cfg.OutputFormat != "mulaw" || cfg.SampleRate != 8000 {
		t.Errorf("synthesis requested %q at %d Hz, want the call's mulaw at 8000", cfg.OutputFormat, cfg.SampleRate)
	}
	reqs := responder.requests()
	if len(reqs) != 1 {
		t.Fatalf("responder asked %d times, want 1", len(reqs))
	}
	req := reqs[0]
	if req.Name != "OmniAgent" || req.Question != "OmniAgent, what did we decide?" ||
		!strings.Contains(req.Transcript.String(), "Let's ship on Friday.") {
		t.Errorf("request = %+v", req)
	}
	if len(rec.Exchanges) != 1 || rec.Exchanges[0].Answer != "We decided to ship on Friday." {
		t.Errorf("Exchanges = %+v", rec.Exchanges)
	}
	last := rec.Transcript[len(rec.Transcript)-1]
	if last.Speaker != "OmniAgent" || last.Text != "We decided to ship on Friday." {
		t.Errorf("last transcript segment = %+v, want the agent's answer", last)
	}
}

func TestAttendAnswersAfterItsNameAlone(t *testing.T) {
	responder := &recordingResponder{reply: "Thursday."}
	sess, _, finish := attendLive(t, Config{Name: "OmniAgent", Speaker: &tinySpeaker{}, Responder: responder})

	sess.admit()
	waitFor(t, "the announcement", func() bool { return sess.clipsSent() == 1 })
	sess.say(t, "Hey Omni Agent?")
	sess.say(t, "When is the launch?")
	waitFor(t, "the answer", func() bool { return sess.clipsSent() == 2 })
	finish()

	if reqs := responder.requests(); len(reqs) != 1 || reqs[0].Question != "Hey Omni Agent? When is the launch?" {
		t.Errorf("requests = %+v", reqs)
	}
}

func TestAttendStaysSilentUnlessAddressedAfterAdmission(t *testing.T) {
	responder := &recordingResponder{reply: "Answer."}
	sess, segs, finish := attendLive(t, Config{Name: "OmniAgent", Speaker: &tinySpeaker{}, Responder: responder})

	// Before admission, even its name gets no answer.
	sess.say(t, "OmniAgent, are you there?")
	waitFor(t, "the first line", func() bool { return segs.count() == 1 })
	sess.admit()
	waitFor(t, "the announcement", func() bool { return sess.clipsSent() == 1 })
	sess.say(t, "We should loop in the agent later.")
	sess.say(t, "OmniAgent, recap please.")
	waitFor(t, "the answer", func() bool { return sess.clipsSent() == 2 })
	rec := finish()

	if reqs := responder.requests(); len(reqs) != 1 || reqs[0].Question != "OmniAgent, recap please." {
		t.Errorf("requests = %+v, want only the question after admission", reqs)
	}
	if len(rec.Exchanges) != 1 {
		t.Errorf("Exchanges = %+v", rec.Exchanges)
	}
}

func TestAttendWithoutSpeakerDoesNotAnswer(t *testing.T) {
	responder := &recordingResponder{reply: "Answer."}
	sess, segs, finish := attendLive(t, Config{Name: "OmniAgent", Responder: responder})

	sess.admit()
	sess.say(t, "OmniAgent, recap please.")
	waitFor(t, "the line", func() bool { return segs.count() == 1 })
	rec := finish()

	if len(responder.requests()) != 0 || len(rec.Exchanges) != 0 || rec.Announced {
		t.Errorf("an agent that cannot speak answered: %+v", rec)
	}
}

func TestRecentTranscriptIsBounded(t *testing.T) {
	at := &attendance{}
	for i := range 100 {
		at.transcript = append(at.transcript, Segment{End: time.Duration(i) * 30 * time.Second, Text: "line"})
	}
	recent := at.recent()
	if len(recent) == 0 || len(recent) > contextSegments {
		t.Fatalf("recent() returned %d segments", len(recent))
	}
	if span := recent[len(recent)-1].End - recent[0].End; span > contextWindow {
		t.Errorf("recent() spans %s, want at most %s", span, contextWindow)
	}
}

func TestPlayTime(t *testing.T) {
	if got := playTime(8000, format.Twilio); got != time.Second {
		t.Errorf("8000 bytes of 8 kHz mu-law = %s, want 1s", got)
	}
	pcm := format.AudioFormat{Encoding: string(format.Linear16), SampleRate: 16000, Channels: 1}
	if got := playTime(16000, pcm); got != 500*time.Millisecond {
		t.Errorf("16000 bytes of 16 kHz PCM = %s, want 500ms", got)
	}
	if got := playTime(100, format.AudioFormat{}); got != 0 {
		t.Errorf("unknown format = %s, want 0", got)
	}
}
