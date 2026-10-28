// Package meeting lets an agent attend a meeting as a note taker: it
// announces itself, transcribes the meeting live, answers when addressed by
// name, and summarizes the meeting with decisions and action items when it
// ends.
//
// The Attendant works on any omnimeet-core attendee.Session that delivers
// audio, such as one from the dial-in joiner:
//
//	sess, err := dialin.NewJoiner(gw).Join(ctx, target)
//	if err != nil {
//		return err
//	}
//	record, err := meeting.New(meeting.Config{
//		Name:        "OmniAgent",
//		Transcriber: deepgram,
//		Speaker:     elevenlabs,
//		Responder:   meeting.LLMResponder{Client: llm, Model: "claude-sonnet-5"},
//		Summarizer:  meeting.LLMSummarizer{Client: llm, Model: "claude-sonnet-5"},
//	}).Attend(ctx, sess)
package meeting

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/plexusone/omnivoice-core/audio/format"
	"github.com/plexusone/omnivoice-core/stt"
	"github.com/plexusone/omnivoice-core/tts"

	"github.com/plexusone/omnimeet-core/attendee"
)

// ErrNoAudio is returned when the session does not deliver meeting audio.
var ErrNoAudio = errors.New("meeting session has no audio to transcribe")

// leaveTimeout bounds leaving the meeting after the caller's context ends.
const leaveTimeout = 15 * time.Second

const (
	// followUpWindow is how long after an utterance that is only the
	// agent's name ("Hey OmniAgent?") the next utterance is taken as the
	// question.
	followUpWindow = 10 * time.Second

	// echoGuard extends the time after the agent speaks during which it
	// does not respond, so its own voice, if the meeting echoes it back,
	// cannot address it.
	echoGuard = 1500 * time.Millisecond

	// contextWindow and contextSegments bound the transcript given to the
	// Responder with a question.
	contextWindow   = 10 * time.Minute
	contextSegments = 80
)

// Transcriber streams speech to text. omnivoice-core's stt.StreamingProvider
// and stt.Client satisfy it.
type Transcriber interface {
	TranscribeStream(ctx context.Context, config stt.TranscriptionConfig) (io.WriteCloser, <-chan stt.StreamEvent, error)
}

// Config configures an Attendant.
type Config struct {
	// Name is how the agent introduces itself, for example "OmniAgent".
	Name string

	// OnBehalfOf names the person the agent attends for, if any.
	OnBehalfOf string

	// Names are other names the agent answers to, besides Name, such as a
	// spelling speech to text tends to produce.
	Names []string

	// Announcement is spoken once when the agent is admitted. Empty uses
	// DefaultAnnouncement, followed by an invitation to ask questions when
	// there is a Responder. It is skipped when Speaker is nil or the session
	// cannot send audio.
	Announcement string

	// Transcriber turns the meeting audio into text. Required.
	Transcriber Transcriber

	// TranscriptionConfig sets the model and language. The encoding and
	// sample rate are taken from the session's audio format.
	TranscriptionConfig stt.TranscriptionConfig

	// Speaker synthesizes the announcement and answers. Optional.
	Speaker tts.Provider

	// SynthesisConfig sets the voice. With no OutputFormat, the meeting's
	// own audio format is requested, such as mu-law at 8 kHz on a phone
	// call, which Deepgram and ElevenLabs produce directly. Otherwise request
	// raw PCM or mu-law output, since compressed formats cannot be played on
	// a call.
	SynthesisConfig tts.SynthesisConfig

	// Responder answers when someone addresses the agent by name, after
	// it is admitted. Optional: without one, or without a Speaker, the agent
	// stays silent after its announcement. It answers one question at a
	// time; a question asked while it is answering is not answered.
	Responder Responder

	// Summarizer summarizes the transcript when the meeting ends. Optional.
	Summarizer Summarizer

	// OnSegment is called for each finalized transcript segment, for
	// example to print a live transcript. Optional.
	OnSegment func(Segment)

	// Logger receives problems that do not stop attendance. Defaults to
	// slog.Default().
	Logger *slog.Logger
}

// Attendant attends meetings as a note taker.
type Attendant struct {
	cfg       Config
	echoGuard time.Duration
}

// New returns an Attendant.
func New(cfg Config) *Attendant {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Name == "" {
		cfg.Name = "OmniAgent"
	}
	return &Attendant{cfg: cfg, echoGuard: echoGuard}
}

// Record is what the Attendant captured from a meeting.
type Record struct {
	// Target is the meeting attended.
	Target attendee.JoinTarget `json:"target"`

	// Started and Ended bound the attendance.
	Started time.Time `json:"started"`
	Ended   time.Time `json:"ended"`

	// FinalState is how the session ended: left, ended, removed, or failed.
	FinalState attendee.State `json:"final_state"`

	// Announced reports whether the announcement was played.
	Announced bool `json:"announced"`

	// Transcript is the meeting's speech. The agent's answers are included
	// with the agent's name as the speaker.
	Transcript Transcript `json:"transcript"`

	// Exchanges are the questions the agent answered.
	Exchanges []Exchange `json:"exchanges,omitempty"`

	// Summary is nil when there is no Summarizer or nothing was said.
	Summary *Summary `json:"summary,omitempty"`
}

// Attend transcribes the session until the meeting ends, then summarizes
// it. When ctx is done the agent leaves the meeting first. Attend returns the
// record even when summarizing fails, together with the error.
func (a *Attendant) Attend(ctx context.Context, sess attendee.Session) (*Record, error) {
	if a.cfg.Transcriber == nil {
		return nil, errors.New("attend meeting: no transcriber")
	}
	rx, ok := sess.(attendee.AudioReceiver)
	if !ok || !sess.Capabilities().ReceiveAudio {
		return nil, ErrNoAudio
	}

	at := &attendance{a: a, sess: sess, started: time.Now()}
	if tx, ok := sess.(attendee.AudioSender); ok && sess.Capabilities().SendAudio && a.cfg.Speaker != nil {
		at.tx = tx
	}
	rec := &Record{Target: sess.Target(), Started: at.started}

	sttConfig := a.cfg.TranscriptionConfig
	in := rx.ReceiveFormat()
	sttConfig.Encoding = string(format.Encoding(in.Encoding).Normalize())
	sttConfig.SampleRate = in.SampleRate
	sttConfig.Channels = max(in.Channels, 1)

	// The STT stream runs until the meeting audio ends, not until ctx ends,
	// so speech before leaving is still transcribed.
	sttCtx, cancelSTT := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelSTT()
	audioIn, events, err := a.cfg.Transcriber.TranscribeStream(sttCtx, sttConfig)
	if err != nil {
		if leaveErr := a.leave(sess); leaveErr != nil {
			return nil, errors.Join(fmt.Errorf("start transcription: %w", err), leaveErr)
		}
		return nil, fmt.Errorf("start transcription: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		a.pumpAudio(rx.ReceiveAudio(), audioIn)
	}()
	go func() {
		defer wg.Done()
		at.collect(ctx, events)
	}()

	rec.Announced, rec.FinalState = at.follow(ctx)
	wg.Wait()
	at.answering.Wait()
	rec.Ended = time.Now()
	rec.Transcript, rec.Exchanges = at.transcript, at.exchanges

	if a.cfg.Summarizer == nil || len(rec.Transcript) == 0 {
		return rec, nil
	}
	summary, err := a.cfg.Summarizer.Summarize(context.WithoutCancel(ctx), rec.Transcript)
	if err != nil {
		return rec, err
	}
	rec.Summary = summary
	return rec, nil
}

// names are the names the agent answers to.
func (a *Attendant) names() []string {
	return append([]string{a.cfg.Name}, a.cfg.Names...)
}

func (a *Attendant) leave(sess attendee.Session) error {
	ctx, cancel := context.WithTimeout(context.Background(), leaveTimeout)
	defer cancel()
	if err := sess.Leave(ctx); err != nil {
		return fmt.Errorf("leave meeting: %w", err)
	}
	return nil
}

// pumpAudio copies meeting audio to the transcriber until the meeting audio
// ends, then closes the transcriber's input so it finishes.
func (a *Attendant) pumpAudio(audio io.Reader, out io.WriteCloser) {
	if _, err := io.Copy(out, audio); err != nil {
		a.cfg.Logger.Error("stream meeting audio to transcriber", "error", err)
	}
	if err := out.Close(); err != nil {
		a.cfg.Logger.Error("close transcriber input", "error", err)
	}
}

// attendance is the state of one Attend call.
type attendance struct {
	a       *Attendant
	sess    attendee.Session
	tx      attendee.AudioSender // nil when the agent cannot speak
	started time.Time

	// admitted is set once the meeting admits the agent; it answers only
	// after that.
	admitted atomic.Bool

	// speaking serializes audio played into the meeting.
	speaking sync.Mutex

	// answering tracks answers in progress, which Attend waits for.
	answering sync.WaitGroup

	mu         sync.Mutex
	transcript Transcript
	exchanges  []Exchange
	busy       bool      // an answer is in progress
	quietUntil time.Time // the agent's own speech may still be heard
	called     *Segment  // an utterance that only named the agent
	calledAt   time.Time
}

// follow watches the session until it ends: it announces the agent once when
// admitted, and leaves when ctx is done. It returns whether the announcement
// played and the final state.
func (at *attendance) follow(ctx context.Context) (bool, attendee.State) {
	announced := false
	leaving := ctx.Done()
	for {
		select {
		case ev, ok := <-at.sess.Events():
			if !ok {
				return announced, at.sess.State()
			}
			switch {
			case ev.Type == attendee.EventError:
				at.a.cfg.Logger.Warn("meeting session error", "session", ev.SessionID, "error", ev.Err)
			case ev.State == attendee.StateAdmitted && !at.admitted.Load():
				// Admitted before announcing: the announcement cannot be
				// heard back until it has played, and say holds off answers
				// until then.
				at.admitted.Store(true)
				announced = at.announce(ctx)
			}
		case <-leaving:
			leaving = nil
			if err := at.a.leave(at.sess); err != nil {
				at.a.cfg.Logger.Error("leave meeting", "error", err)
			}
		}
	}
}

// announce plays the disclosure, if the agent can speak.
func (at *attendance) announce(ctx context.Context) bool {
	if at.tx == nil {
		return false
	}
	cfg := at.a.cfg
	text := cfg.Announcement
	if text == "" {
		text = DefaultAnnouncement(cfg.Name, cfg.OnBehalfOf)
		if cfg.Responder != nil {
			text += answerInvitation
		}
	}
	if _, err := at.say(ctx, text); err != nil {
		cfg.Logger.Error("announce in meeting", "error", err)
		return false
	}
	return true
}

// say plays text into the meeting and keeps the agent from responding until
// the speech, and any echo of it, is over.
func (at *attendance) say(ctx context.Context, text string) (time.Duration, error) {
	at.speaking.Lock()
	defer at.speaking.Unlock()
	cfg := at.a.cfg
	out := at.tx.SendFormat()
	synth := cfg.SynthesisConfig
	if synth.OutputFormat == "" {
		synth.OutputFormat = string(format.Encoding(out.Encoding).Normalize())
		synth.SampleRate = out.SampleRate
	}
	d, err := speak(ctx, cfg.Speaker, synth, text, at.tx.SendAudio(), out)
	if err != nil {
		return 0, err
	}
	at.mu.Lock()
	at.quietUntil = time.Now().Add(d + at.a.echoGuard)
	at.mu.Unlock()
	return d, nil
}

// collect gathers final transcript segments until the transcriber finishes,
// and answers those that address the agent.
func (at *attendance) collect(ctx context.Context, events <-chan stt.StreamEvent) {
	cfg := at.a.cfg
	for ev := range events {
		switch ev.Type {
		case stt.EventError:
			cfg.Logger.Warn("transcription error", "error", ev.Error)
			continue
		case stt.EventTranscript:
		default:
			continue
		}
		if !ev.IsFinal || ev.Transcript == "" {
			continue
		}
		seg := Segment{Text: ev.Transcript}
		if ev.Segment != nil {
			seg.Start, seg.End, seg.Speaker = ev.Segment.StartTime, ev.Segment.EndTime, ev.Segment.Speaker
		} else {
			seg.Start = time.Since(at.started)
			seg.End = seg.Start
		}
		at.mu.Lock()
		at.transcript = append(at.transcript, seg)
		at.mu.Unlock()
		if cfg.OnSegment != nil {
			cfg.OnSegment(seg)
		}
		at.consider(ctx, seg)
	}
}

// consider starts an answer when seg addresses the agent and it can answer.
func (at *attendance) consider(ctx context.Context, seg Segment) {
	if at.a.cfg.Responder == nil || at.tx == nil || !at.admitted.Load() || ctx.Err() != nil {
		return
	}
	at.mu.Lock()
	defer at.mu.Unlock()
	if time.Now().Before(at.quietUntil) {
		return
	}
	question, ok := at.question(seg)
	if !ok {
		return
	}
	if at.busy {
		at.a.cfg.Logger.Info("not answering while another answer is in progress", "question", question)
		return
	}
	at.busy = true
	recent := at.recent()
	at.answering.Add(1)
	go at.answer(ctx, question, recent)
}

// question returns the question seg asks the agent, if any. An utterance
// that is only the agent's name waits for the next one. at.mu is held.
func (at *attendance) question(seg Segment) (string, bool) {
	ok, nameOnly := addressed(seg.Text, at.a.names())
	switch {
	case ok && nameOnly:
		at.called, at.calledAt = &seg, time.Now()
		return "", false
	case ok:
		at.called = nil
		return seg.Text, true
	case at.called != nil && time.Since(at.calledAt) <= followUpWindow:
		q := at.called.Text + " " + seg.Text
		at.called = nil
		return q, true
	default:
		at.called = nil
		return "", false
	}
}

// recent returns the end of the transcript for a question's context. at.mu
// is held.
func (at *attendance) recent() Transcript {
	t := at.transcript
	if len(t) > contextSegments {
		t = t[len(t)-contextSegments:]
	}
	if n := len(t); n > 0 {
		cutoff := t[n-1].End - contextWindow
		i := 0
		for i < n-1 && t[i].End < cutoff {
			i++
		}
		t = t[i:]
	}
	return append(Transcript(nil), t...)
}

// answer asks the Responder and speaks its answer.
func (at *attendance) answer(ctx context.Context, question string, recent Transcript) {
	defer at.answering.Done()
	defer func() {
		at.mu.Lock()
		at.busy = false
		at.mu.Unlock()
	}()
	cfg := at.a.cfg
	reply, err := cfg.Responder.Respond(ctx, Request{Name: cfg.Name, Question: question, Transcript: recent})
	if err != nil {
		cfg.Logger.Error("answer in meeting", "error", err)
		return
	}
	reply = strings.TrimSpace(reply)
	if reply == "" || ctx.Err() != nil {
		return
	}
	start := time.Since(at.started)
	d, err := at.say(ctx, reply)
	if err != nil {
		cfg.Logger.Error("speak answer in meeting", "error", err)
		return
	}
	seg := Segment{Start: start, End: start + d, Speaker: cfg.Name, Text: reply}
	at.mu.Lock()
	at.transcript = append(at.transcript, seg)
	at.exchanges = append(at.exchanges, Exchange{At: start, Question: question, Answer: reply})
	at.mu.Unlock()
	if cfg.OnSegment != nil {
		cfg.OnSegment(seg)
	}
}
