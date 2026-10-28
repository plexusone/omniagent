package meeting

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/plexusone/omnillm-core/provider"
	"github.com/plexusone/omnivoice-core/stt"
	"github.com/plexusone/omnivoice-core/tts"

	"github.com/plexusone/omnimeet-core/attendee"
	"github.com/plexusone/omnimeet-core/dialin"
	"github.com/plexusone/omnimeet-core/dialin/dialintest"
)

// textTranscriber treats each audio chunk as the text that was spoken.
type textTranscriber struct {
	config stt.TranscriptionConfig
}

func (t *textTranscriber) TranscribeStream(_ context.Context, config stt.TranscriptionConfig) (io.WriteCloser, <-chan stt.StreamEvent, error) {
	t.config = config
	events := make(chan stt.StreamEvent, 16)
	return &textStream{events: events}, events, nil
}

type textStream struct {
	events chan stt.StreamEvent
	once   sync.Once
}

func (s *textStream) Write(p []byte) (int, error) {
	s.events <- stt.StreamEvent{Type: stt.EventTranscript, Transcript: strings.TrimSpace(string(p)), IsFinal: true}
	return len(p), nil
}

func (s *textStream) Close() error {
	s.once.Do(func() { close(s.events) })
	return nil
}

// pcmSpeaker returns a short 16 kHz PCM clip for any text.
type pcmSpeaker struct {
	text string
}

func (p *pcmSpeaker) Name() string { return "fake" }
func (p *pcmSpeaker) Synthesize(_ context.Context, text string, _ tts.SynthesisConfig) (*tts.SynthesisResult, error) {
	p.text = text
	return &tts.SynthesisResult{Audio: make([]byte, 3200), Format: "pcm_16000"}, nil
}
func (p *pcmSpeaker) SynthesizeStream(context.Context, string, tts.SynthesisConfig) (<-chan tts.StreamChunk, error) {
	return nil, errors.New("not supported")
}
func (p *pcmSpeaker) ListVoices(context.Context) ([]tts.Voice, error)      { return nil, nil }
func (p *pcmSpeaker) GetVoice(context.Context, string) (*tts.Voice, error) { return nil, nil }

// jsonLLM replies with a fenced JSON summary and records the request.
type jsonLLM struct {
	reply   string
	err     error
	request *provider.ChatCompletionRequest
}

func (l *jsonLLM) CreateChatCompletion(_ context.Context, req *provider.ChatCompletionRequest) (*provider.ChatCompletionResponse, error) {
	l.request = req
	if l.err != nil {
		return nil, l.err
	}
	return &provider.ChatCompletionResponse{
		Choices: []provider.ChatCompletionChoice{{Message: provider.Message{Role: provider.RoleAssistant, Content: l.reply}}},
	}, nil
}

const summaryReply = "```json\n" + `{"overview": "The team set the launch date.",
 "decisions": ["Launch on Thursday"],
 "action_items": [{"task": "Update the roadmap", "owner": "Dana"}],
 "open_questions": []}` + "\n```"

func joinSimulated(t *testing.T, meeting *dialintest.Meeting) attendee.Session {
	t.Helper()
	target := attendee.JoinTarget{
		Platform: meeting.Platform,
		DialIn: &attendee.DialInDetails{
			Numbers:      []attendee.PhoneNumber{{Number: meeting.Number, Country: "US"}},
			ConferenceID: meeting.ConferenceID,
		},
	}
	sess, err := dialin.NewJoiner(dialintest.NewGateway(meeting),
		dialin.WithTiming(dialin.Timing{Answer: time.Millisecond, Between: time.Millisecond}),
		dialin.WithKeyDuration(0),
	).Join(context.Background(), target)
	if err != nil {
		t.Fatalf("Join() error: %v", err)
	}
	return sess
}

func newMeeting(script ...string) *dialintest.Meeting {
	m := &dialintest.Meeting{Platform: attendee.PlatformGoogleMeet, Number: "+14135550123", ConferenceID: "123456789"}
	for _, line := range script {
		m.Script = append(m.Script, []byte(line))
	}
	return m
}

func TestAttendTranscribesAnnouncesAndSummarizes(t *testing.T) {
	meeting := newMeeting("Let's launch on Thursday.", "Dana will update the roadmap.")
	sess := joinSimulated(t, meeting)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var live []string
	speaker := &pcmSpeaker{}
	llm := &jsonLLM{reply: summaryReply}
	transcriber := &textTranscriber{}

	attendant := New(Config{
		Name:        "OmniAgent",
		OnBehalfOf:  "Jane",
		Transcriber: transcriber,
		Speaker:     speaker,
		Summarizer:  LLMSummarizer{Client: llm, Model: "test-model"},
		OnSegment: func(s Segment) {
			mu.Lock()
			defer mu.Unlock()
			live = append(live, s.Text)
		},
	})

	// Leave once the meeting has been heard and the announcement played.
	go func() {
		for ctx.Err() == nil {
			mu.Lock()
			heard := len(live) == 2
			mu.Unlock()
			if heard && len(meeting.Received()) > 0 {
				cancel()
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
	rec, err := attendant.Attend(ctx, sess)
	if err != nil {
		t.Fatalf("Attend() error: %v", err)
	}

	if transcriber.config.Encoding != "mulaw" || transcriber.config.SampleRate != 8000 {
		t.Errorf("transcription config = %+v, want mulaw 8 kHz from the session", transcriber.config)
	}
	if got := rec.Transcript.String(); !strings.Contains(got, "Let's launch on Thursday.") ||
		!strings.Contains(got, "Dana will update the roadmap.") {
		t.Errorf("transcript = %q", got)
	}
	if rec.FinalState != attendee.StateLeft {
		t.Errorf("FinalState = %s, want left", rec.FinalState)
	}
	if calls := meeting.Calls(); len(calls) != 1 || !calls[0].HungUp() {
		t.Error("the call was not hung up after leaving")
	}

	if !rec.Announced || !strings.Contains(speaker.text, "attending for Jane") {
		t.Errorf("Announced = %v with %q", rec.Announced, speaker.text)
	}
	received := meeting.Received()
	if len(received) != 1 || len(received[0]) != 800 {
		t.Errorf("meeting received %d clips, want one 0.1 s mu-law clip (800 bytes)", len(received))
	}

	if rec.Summary == nil || rec.Summary.Decisions[0] != "Launch on Thursday" ||
		rec.Summary.ActionItems[0].Owner != "Dana" {
		t.Fatalf("Summary = %+v", rec.Summary)
	}
	if llm.request.Model != "test-model" || !strings.Contains(llm.request.Messages[1].Content, "Let's launch on Thursday.") {
		t.Errorf("summary request = %+v", llm.request)
	}
	if !strings.Contains(llm.request.Messages[0].Content, "not instructions") {
		t.Error("summary prompt does not mark the transcript as untrusted data")
	}
}

func TestAttendUntilMeetingEnds(t *testing.T) {
	meeting := newMeeting("Short sync.")
	meeting.EndAfterScript = true
	sess := joinSimulated(t, meeting)

	rec, err := New(Config{Transcriber: &textTranscriber{}}).Attend(context.Background(), sess)
	if err != nil {
		t.Fatalf("Attend() error: %v", err)
	}
	if rec.FinalState != attendee.StateEnded {
		t.Errorf("FinalState = %s, want ended", rec.FinalState)
	}
	if len(rec.Transcript) != 1 || rec.Transcript[0].Text != "Short sync." {
		t.Errorf("Transcript = %+v", rec.Transcript)
	}
	if rec.Summary != nil || rec.Announced {
		t.Errorf("Summary = %+v, Announced = %v; want neither without a summarizer or speaker", rec.Summary, rec.Announced)
	}
}

func TestAttendReturnsRecordWhenSummaryFails(t *testing.T) {
	meeting := newMeeting("One line.")
	meeting.EndAfterScript = true
	sess := joinSimulated(t, meeting)

	llmErr := errors.New("model unavailable")
	rec, err := New(Config{
		Transcriber: &textTranscriber{},
		Summarizer:  LLMSummarizer{Client: &jsonLLM{err: llmErr}},
	}).Attend(context.Background(), sess)
	if !errors.Is(err, llmErr) {
		t.Fatalf("Attend() error = %v, want the summarizer error", err)
	}
	if rec == nil || len(rec.Transcript) != 1 {
		t.Fatalf("record = %+v, want the transcript despite the error", rec)
	}
}

// silentSession is a session without audio.
type silentSession struct{}

func (silentSession) ID() string                          { return "s" }
func (silentSession) Target() attendee.JoinTarget         { return attendee.JoinTarget{} }
func (silentSession) Capabilities() attendee.Capabilities { return attendee.Capabilities{} }
func (silentSession) State() attendee.State               { return attendee.StateAdmitted }
func (silentSession) Events() <-chan attendee.Event       { return nil }
func (silentSession) Leave(context.Context) error         { return nil }

func TestAttendRequiresAudio(t *testing.T) {
	if _, err := New(Config{Transcriber: &textTranscriber{}}).Attend(context.Background(), silentSession{}); !errors.Is(err, ErrNoAudio) {
		t.Fatalf("Attend() error = %v, want ErrNoAudio", err)
	}
}
