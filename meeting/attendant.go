// Package meeting lets an agent attend a meeting as a note taker: it
// announces itself, transcribes the meeting live, and summarizes it with
// decisions and action items when the meeting ends.
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
//		Summarizer:  meeting.LLMSummarizer{Client: llm, Model: "claude-sonnet-5"},
//	}).Attend(ctx, sess)
package meeting

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
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

	// Announcement is spoken once when the agent is admitted. Empty uses
	// DefaultAnnouncement. It is skipped when Speaker is nil or the session
	// cannot send audio.
	Announcement string

	// Transcriber turns the meeting audio into text. Required.
	Transcriber Transcriber

	// TranscriptionConfig sets the model and language. The encoding and
	// sample rate are taken from the session's audio format.
	TranscriptionConfig stt.TranscriptionConfig

	// Speaker synthesizes the announcement. Optional.
	Speaker tts.Provider

	// SynthesisConfig sets the voice. Request raw PCM or mu-law output, since
	// compressed formats cannot be played on a call.
	SynthesisConfig tts.SynthesisConfig

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
	cfg Config
}

// New returns an Attendant.
func New(cfg Config) *Attendant {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Name == "" {
		cfg.Name = "OmniAgent"
	}
	return &Attendant{cfg: cfg}
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

	// Transcript is the meeting's speech.
	Transcript Transcript `json:"transcript"`

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

	rec := &Record{Target: sess.Target(), Started: time.Now()}

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
		rec.Transcript = a.collect(events, rec.Started)
	}()

	rec.Announced, rec.FinalState = a.follow(ctx, sess)
	wg.Wait()
	rec.Ended = time.Now()

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

// follow watches the session until it ends: it announces the agent once when
// admitted, and leaves when ctx is done. It returns whether the announcement
// played and the final state.
func (a *Attendant) follow(ctx context.Context, sess attendee.Session) (bool, attendee.State) {
	announced := false
	leaving := ctx.Done()
	for {
		select {
		case ev, ok := <-sess.Events():
			if !ok {
				return announced, sess.State()
			}
			switch {
			case ev.Type == attendee.EventError:
				a.cfg.Logger.Warn("meeting session error", "session", ev.SessionID, "error", ev.Err)
			case ev.State == attendee.StateAdmitted && !announced:
				announced = a.announce(ctx, sess)
			}
		case <-leaving:
			leaving = nil
			if err := a.leave(sess); err != nil {
				a.cfg.Logger.Error("leave meeting", "error", err)
			}
		}
	}
}

// announce plays the disclosure, if the agent can speak.
func (a *Attendant) announce(ctx context.Context, sess attendee.Session) bool {
	tx, ok := sess.(attendee.AudioSender)
	if !ok || !sess.Capabilities().SendAudio || a.cfg.Speaker == nil {
		return false
	}
	text := a.cfg.Announcement
	if text == "" {
		text = DefaultAnnouncement(a.cfg.Name, a.cfg.OnBehalfOf)
	}
	if err := speak(ctx, a.cfg.Speaker, a.cfg.SynthesisConfig, text, tx.SendAudio(), tx.SendFormat()); err != nil {
		a.cfg.Logger.Error("announce in meeting", "error", err)
		return false
	}
	return true
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

// collect gathers final transcript segments until the transcriber finishes.
func (a *Attendant) collect(events <-chan stt.StreamEvent, started time.Time) Transcript {
	var transcript Transcript
	for ev := range events {
		switch ev.Type {
		case stt.EventError:
			a.cfg.Logger.Warn("transcription error", "error", ev.Error)
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
			seg.Start = time.Since(started)
			seg.End = seg.Start
		}
		transcript = append(transcript, seg)
		if a.cfg.OnSegment != nil {
			a.cfg.OnSegment(seg)
		}
	}
	return transcript
}
