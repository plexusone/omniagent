package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/plexusone/omnillm"
	"github.com/plexusone/omnivoice"
	"github.com/plexusone/omnivoice-core/gateway"
	"github.com/plexusone/omnivoice-core/stt"
	"github.com/plexusone/omnivoice-core/tts"

	"github.com/plexusone/omnimeet-core/attendee"
	"github.com/plexusone/omnimeet-core/dialin"
	"github.com/plexusone/omnimeet-core/invite"

	"github.com/plexusone/omniagent/meeting"
	"github.com/plexusone/omniagent/voice"
)

var meetingCmd = &cobra.Command{
	Use:   "meeting",
	Short: "Attend meetings hosted by others",
	Long: `Attend Zoom, Google Meet, and Microsoft Teams meetings hosted by others.

Join a meeting by phone from its invitation:
  omniagent meeting join --invite meeting.ics --ngrok`,
}

var meetingJoinCmd = &cobra.Command{
	Use:   "join",
	Short: "Join a meeting by phone, transcribe it, and write notes",
	Long: `Join a meeting by phone dial-in from its invitation, announce the agent,
print the transcript as the meeting goes, and write meeting notes with a
summary, decisions, and action items when it ends.

The invitation is an email or calendar description saved as text, or an .ics
file; use "-" to read it from standard input. The meeting must list a phone
dial-in number. Press Ctrl+C to leave early.

The agent announces that it is an AI assistant transcribing the meeting when
it is admitted. Get consent from the meeting's participants before recording.

Required environment variables:
  Twilio:  TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN
  Telnyx:  TELNYX_API_KEY, TELNYX_CONNECTION_ID
  NGROK_AUTHTOKEN       with --ngrok
  DEEPGRAM_API_KEY      for the default speech-to-text provider
  ELEVENLABS_API_KEY    for the default text-to-speech provider
  ANTHROPIC_API_KEY or OPENAI_API_KEY for the summary

Examples:
  omniagent meeting join --invite meeting.ics --phone +14155550199 --ngrok
  pbpaste | omniagent meeting join --invite - --public-url https://agent.example.com`,
	RunE: runMeetingJoin,
}

// meetingJoinOptions holds the meeting join flags.
type meetingJoinOptions struct {
	Invite      string
	Provider    string
	Phone       string
	PublicURL   string
	Listen      string
	Ngrok       bool
	NgrokDomain string
	Country     string
	Name        string
	OnBehalfOf  string
	NoAnnounce  bool
	STT         string
	STTModel    string
	Language    string
	TTS         string
	Voice       string
	LLM         string
	Model       string
	NoSummary   bool
	Out         string
	JSONOut     string
	MaxDuration time.Duration
}

var meetingJoinOpts meetingJoinOptions

func init() {
	f := meetingJoinCmd.Flags()
	o := &meetingJoinOpts
	f.StringVar(&o.Invite, "invite", "", `Invitation text or .ics file, or "-" for standard input (required)`)
	f.StringVar(&o.Provider, "provider", "twilio", "Telephony provider (twilio, telnyx)")
	f.StringVar(&o.Phone, "phone", "", "Caller ID for the call, in E.164 form (default: TWILIO_PHONE_NUMBER or TELNYX_PHONE_NUMBER)")
	f.StringVar(&o.PublicURL, "public-url", "", "Public URL where the provider reaches this machine (or use --ngrok)")
	f.StringVar(&o.Listen, "listen", ":8080", "Address to listen on for the provider's webhooks")
	f.BoolVar(&o.Ngrok, "ngrok", false, "Open an ngrok tunnel for the public URL (requires NGROK_AUTHTOKEN)")
	f.StringVar(&o.NgrokDomain, "ngrok-domain", "", "Custom ngrok domain")
	f.StringVar(&o.Country, "country", "US", "Country of the dial-in number to call (ISO 3166-1 alpha-2)")
	f.StringVar(&o.Name, "name", "OmniAgent", "Name the agent announces itself with")
	f.StringVar(&o.OnBehalfOf, "on-behalf-of", "", "Person the agent attends for, named in the announcement")
	f.BoolVar(&o.NoAnnounce, "no-announce", false, "Do not announce the agent when it joins")
	f.StringVar(&o.STT, "stt", "deepgram", "Streaming speech-to-text provider")
	f.StringVar(&o.STTModel, "stt-model", "", "Speech-to-text model")
	f.StringVar(&o.Language, "language", "", "Meeting language, for example en-US")
	f.StringVar(&o.TTS, "tts", "elevenlabs", "Text-to-speech provider for the announcement")
	f.StringVar(&o.Voice, "voice", "", "Text-to-speech voice ID")
	f.StringVar(&o.LLM, "llm", "", "LLM provider for the summary (anthropic, openai, gemini); detected from API keys if empty")
	f.StringVar(&o.Model, "model", "", "LLM model for the summary")
	f.BoolVar(&o.NoSummary, "no-summary", false, "Skip the summary")
	f.StringVar(&o.Out, "out", "", "Meeting notes Markdown file (default: meeting-<time>.md)")
	f.StringVar(&o.JSONOut, "json", "", "Also write the full record as JSON to this file")
	f.DurationVar(&o.MaxDuration, "max-duration", 2*time.Hour, "Leave the meeting after this long")
	if err := meetingJoinCmd.MarkFlagRequired("invite"); err != nil {
		panic(err) // the flag is defined above
	}

	meetingCmd.AddCommand(meetingJoinCmd)
	rootCmd.AddCommand(meetingCmd)
}

func runMeetingJoin(cmd *cobra.Command, _ []string) error {
	o := meetingJoinOpts
	if !o.Ngrok && o.PublicURL == "" {
		return errors.New("--public-url is required for the provider's webhooks (or use --ngrok)")
	}
	target, err := readMeetingInvite(o.Invite, cmd.InOrStdin())
	if err != nil {
		return err
	}
	logger := slog.Default()

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, o.MaxDuration)
	defer cancel()

	transcriber, err := newMeetingTranscriber(o)
	if err != nil {
		return err
	}
	var speaker tts.Provider
	if !o.NoAnnounce {
		if speaker, err = newMeetingSpeaker(o); err != nil {
			return fmt.Errorf("%w (or use --no-announce)", err)
		}
	}
	var summarizer meeting.Summarizer
	if !o.NoSummary {
		if summarizer, err = newMeetingSummarizer(o); err != nil {
			return fmt.Errorf("%w (or use --no-summary)", err)
		}
	}

	publicURL := o.PublicURL
	var listener net.Listener
	if o.Ngrok {
		if listener, publicURL, err = createNgrokTunnel(ctx, logger, o.NgrokDomain); err != nil {
			return fmt.Errorf("create ngrok tunnel: %w", err)
		}
		defer func() {
			if err := listener.Close(); err != nil {
				logger.Warn("close ngrok tunnel", "error", err)
			}
		}()
	}

	gw, err := voice.NewAudioGateway(voice.AudioGatewayConfig{
		Provider:    o.Provider,
		PhoneNumber: firstSet(o.Phone, os.Getenv(strings.ToUpper(o.Provider)+"_PHONE_NUMBER")),
		PublicURL:   publicURL,
		ListenAddr:  o.Listen,
		Listener:    listener,
		Logger:      logger,
	})
	if err != nil {
		return err
	}
	gwCtx, stopGateway := context.WithCancel(context.WithoutCancel(ctx))
	defer stopGateway()
	gwErr := make(chan error, 1)
	go func() { gwErr <- gw.Start(gwCtx) }()
	// Fail fast if the gateway cannot start, for example when the port is
	// in use, rather than placing a call the provider cannot connect.
	select {
	case err := <-gwErr:
		return fmt.Errorf("start voice gateway: %w", err)
	case <-time.After(gatewayStartGrace):
	}

	out := cmd.OutOrStdout()
	printOut(out, "Joining %s meeting %s by phone...\n", target.Platform, target.JoinURL)
	record, err := attendMeeting(ctx, meetingDeps{
		Gateway:     gw,
		Transcriber: transcriber,
		Speaker:     speaker,
		Summarizer:  summarizer,
	}, target, o, out)

	stopGateway()
	if gerr := <-gwErr; gerr != nil {
		logger.Warn("voice gateway stopped", "error", gerr)
	}
	if record == nil {
		return err
	}
	if werr := writeMeetingRecord(record, o, out); werr != nil {
		return errors.Join(err, werr)
	}
	return err
}

// meetingDeps are the services attendMeeting uses, so tests can substitute
// a simulated gateway and fake speech and LLM clients.
type meetingDeps struct {
	Gateway     gateway.Gateway
	Transcriber meeting.Transcriber
	Speaker     tts.Provider
	Summarizer  meeting.Summarizer
	JoinOptions []dialin.JoinerOption
}

// attendMeeting joins the meeting by phone and attends it until it ends or
// ctx is done, printing transcript lines to out as they arrive.
func attendMeeting(ctx context.Context, deps meetingDeps, target attendee.JoinTarget,
	o meetingJoinOptions, out io.Writer) (*meeting.Record, error) {
	joinOpts := append([]dialin.JoinerOption{dialin.WithCountry(o.Country)}, deps.JoinOptions...)
	sess, err := dialin.NewJoiner(deps.Gateway, joinOpts...).Join(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("join meeting: %w", err)
	}

	cfg := meeting.Config{
		Name:                o.Name,
		OnBehalfOf:          o.OnBehalfOf,
		Transcriber:         deps.Transcriber,
		TranscriptionConfig: stt.TranscriptionConfig{Model: o.STTModel, Language: o.Language, EnablePunctuation: true},
		Speaker:             deps.Speaker,
		SynthesisConfig:     synthesisConfig(o.TTS, o.Voice),
		Summarizer:          deps.Summarizer,
		OnSegment: func(s meeting.Segment) {
			printOut(out, "%s", meeting.Transcript{s}.String())
		},
	}
	return meeting.New(cfg).Attend(ctx, sess)
}

// readMeetingInvite parses an invitation file, an .ics file, or standard
// input when path is "-".
func readMeetingInvite(path string, stdin io.Reader) (attendee.JoinTarget, error) {
	var (
		r   io.Reader = stdin
		ics bool
	)
	if path != "-" {
		f, err := os.Open(path) //nolint:gosec // the invite path is chosen by the user
		if err != nil {
			return attendee.JoinTarget{}, fmt.Errorf("open invite: %w", err)
		}
		defer func() {
			if err := f.Close(); err != nil {
				slog.Default().Warn("close invite", "error", err)
			}
		}()
		r = f
		ics = strings.HasSuffix(strings.ToLower(path), ".ics")
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return attendee.JoinTarget{}, fmt.Errorf("read invite: %w", err)
	}
	if ics || strings.Contains(string(data), "BEGIN:VCALENDAR") {
		return invite.ParseICS(strings.NewReader(string(data)))
	}
	return invite.Parse(string(data))
}

// writeMeetingRecord writes the meeting notes, and the JSON record when asked,
// and prints the summary.
func writeMeetingRecord(rec *meeting.Record, o meetingJoinOptions, out io.Writer) error {
	path := o.Out
	if path == "" {
		path = "meeting-" + rec.Started.Format("20060102-150405") + ".md"
	}
	if err := os.WriteFile(path, []byte(rec.Markdown()), 0o600); err != nil {
		return fmt.Errorf("write meeting notes: %w", err)
	}
	if rec.Summary != nil {
		printOut(out, "\n%s", rec.Summary.Markdown())
	}
	printOut(out, "\nMeeting notes written to %s\n", path)

	if o.JSONOut == "" {
		return nil
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("encode meeting record: %w", err)
	}
	if err := os.WriteFile(o.JSONOut, data, 0o600); err != nil {
		return fmt.Errorf("write meeting record: %w", err)
	}
	return nil
}

// newMeetingTranscriber returns the streaming speech-to-text provider.
func newMeetingTranscriber(o meetingJoinOptions) (meeting.Transcriber, error) {
	key, err := providerAPIKey(o.STT)
	if err != nil {
		return nil, fmt.Errorf("speech-to-text: %w", err)
	}
	p, err := omnivoice.GetSTTProvider(o.STT, omnivoice.WithAPIKey(key))
	if err != nil {
		return nil, fmt.Errorf("speech-to-text provider %q: %w", o.STT, err)
	}
	streaming, ok := p.(stt.StreamingProvider)
	if !ok {
		return nil, fmt.Errorf("speech-to-text provider %q does not support streaming", o.STT)
	}
	return streaming, nil
}

// newMeetingSpeaker returns the text-to-speech provider for the announcement.
func newMeetingSpeaker(o meetingJoinOptions) (tts.Provider, error) {
	key, err := providerAPIKey(o.TTS)
	if err != nil {
		return nil, fmt.Errorf("text-to-speech for the announcement: %w", err)
	}
	p, err := omnivoice.GetTTSProvider(o.TTS, omnivoice.WithAPIKey(key))
	if err != nil {
		return nil, fmt.Errorf("text-to-speech provider %q: %w", o.TTS, err)
	}
	return p, nil
}

// newMeetingSummarizer returns an LLM summarizer.
func newMeetingSummarizer(o meetingJoinOptions) (meeting.Summarizer, error) {
	llm := o.LLM
	if llm == "" {
		switch {
		case os.Getenv("ANTHROPIC_API_KEY") != "":
			llm = "anthropic"
		case os.Getenv("OPENAI_API_KEY") != "":
			llm = "openai"
		case os.Getenv("GEMINI_API_KEY") != "":
			llm = "gemini"
		default:
			return nil, errors.New("summary: no LLM provider; set ANTHROPIC_API_KEY, OPENAI_API_KEY, or GEMINI_API_KEY")
		}
	}
	key, err := providerAPIKey(llm)
	if err != nil {
		return nil, fmt.Errorf("summary: %w", err)
	}
	client, err := omnillm.NewClient(omnillm.ClientConfig{
		Providers: []omnillm.ProviderConfig{{Provider: omnillm.ProviderName(llm), APIKey: key}},
	})
	if err != nil {
		return nil, fmt.Errorf("summary: create LLM client: %w", err)
	}
	model := o.Model
	if model == "" {
		model = defaultSummaryModel(llm)
	}
	return meeting.LLMSummarizer{Client: client, Model: model}, nil
}

func defaultSummaryModel(llm string) string {
	switch llm {
	case "openai":
		return "gpt-4o"
	case "gemini":
		return "gemini-2.5-flash"
	default:
		return "claude-sonnet-5"
	}
}

// providerAPIKeyEnv names the environment variable holding each provider's
// API key.
var providerAPIKeyEnv = map[string]string{
	"deepgram":   "DEEPGRAM_API_KEY",
	"elevenlabs": "ELEVENLABS_API_KEY",
	"openai":     "OPENAI_API_KEY",
	"anthropic":  "ANTHROPIC_API_KEY",
	"gemini":     "GEMINI_API_KEY",
	"google":     "GOOGLE_API_KEY",
	"assemblyai": "ASSEMBLYAI_API_KEY",
}

// providerAPIKey reads a provider's API key from its environment variable.
func providerAPIKey(provider string) (string, error) {
	env, ok := providerAPIKeyEnv[provider]
	if !ok {
		env = strings.ToUpper(provider) + "_API_KEY"
	}
	key := os.Getenv(env)
	if key == "" {
		return "", fmt.Errorf("%s is not set for provider %q", env, provider)
	}
	return key, nil
}

// synthesisConfig requests audio a call can play: mu-law at 8 kHz where the
// provider offers it, raw PCM otherwise.
func synthesisConfig(provider, voiceID string) tts.SynthesisConfig {
	cfg := tts.SynthesisConfig{VoiceID: voiceID}
	switch provider {
	case "elevenlabs":
		cfg.OutputFormat = "ulaw_8000"
	default:
		cfg.OutputFormat = "pcm"
		cfg.SampleRate = 24000
	}
	return cfg
}

// gatewayStartGrace is how long to watch for the voice gateway failing to
// start before placing the call.
const gatewayStartGrace = 250 * time.Millisecond

// printOut writes progress to the command's output. A failed write, such as
// a closed pipe, is logged rather than stopping the meeting.
func printOut(out io.Writer, format string, args ...any) {
	if _, err := fmt.Fprintf(out, format, args...); err != nil {
		slog.Default().Warn("write meeting output", "error", err)
	}
}

func firstSet(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
