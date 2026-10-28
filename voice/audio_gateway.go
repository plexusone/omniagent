package voice

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/plexusone/omnivoice-core/gateway"

	telnyxgateway "github.com/plexusone/omni-telnyx/omnivoice/gateway"
	twiliogateway "github.com/plexusone/omni-twilio/omnivoice/gateway"
)

// AudioGatewayConfig configures a voice gateway in audio mode.
type AudioGatewayConfig struct {
	// Provider is "twilio" (the default) or "telnyx".
	Provider string

	// Twilio credentials; default to TWILIO_ACCOUNT_SID and TWILIO_AUTH_TOKEN.
	TwilioAccountSID string
	TwilioAuthToken  string //nolint:gosec // G117: credential loaded from the environment

	// Telnyx credentials; default to TELNYX_API_KEY and TELNYX_CONNECTION_ID.
	TelnyxAPIKey       string //nolint:gosec // G117: credential loaded from the environment
	TelnyxConnectionID string

	// PhoneNumber is the caller ID for outbound calls, in E.164 form.
	PhoneNumber string

	// PublicURL is where the provider reaches the gateway's webhooks and
	// media stream, for example "https://your-server.com".
	PublicURL string

	// ListenAddr is the local address; defaults to ":8080".
	ListenAddr string

	// Listener replaces ListenAddr, for example an ngrok tunnel.
	Listener net.Listener

	// MaxSessionDuration ends calls that run longer; defaults to 2 hours.
	MaxSessionDuration time.Duration

	Logger *slog.Logger
}

// NewAudioGateway returns a Twilio or Telnyx gateway in
// gateway.PipelineModeAudio: it runs no STT, LLM, or TTS pipeline and hands
// each call's audio to the application, as the dial-in meeting joiner needs.
// The gateway accepts call options (gateway.OptionCaller) such as keypad
// digits. Start it before placing calls.
func NewAudioGateway(cfg AudioGatewayConfig) (gateway.Gateway, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.MaxSessionDuration == 0 {
		cfg.MaxSessionDuration = 2 * time.Hour
	}
	if cfg.PublicURL == "" {
		return nil, fmt.Errorf("audio gateway: public URL is required for the provider's webhooks")
	}

	switch cfg.Provider {
	case "", "twilio":
		gw, err := twiliogateway.New(twiliogateway.Config{
			AccountSID:         firstNonEmpty(cfg.TwilioAccountSID, os.Getenv("TWILIO_ACCOUNT_SID")),
			AuthToken:          firstNonEmpty(cfg.TwilioAuthToken, os.Getenv("TWILIO_AUTH_TOKEN")),
			PhoneNumber:        cfg.PhoneNumber,
			PublicURL:          cfg.PublicURL,
			ListenAddr:         cfg.ListenAddr,
			Listener:           cfg.Listener,
			Mode:               gateway.PipelineModeAudio,
			MaxSessionDuration: cfg.MaxSessionDuration,
			Logger:             cfg.Logger,
		})
		if err != nil {
			return nil, fmt.Errorf("create twilio audio gateway: %w", err)
		}
		return gw, nil
	case "telnyx":
		gw, err := telnyxgateway.New(telnyxgateway.Config{
			APIKey:             firstNonEmpty(cfg.TelnyxAPIKey, os.Getenv("TELNYX_API_KEY")),
			ConnectionID:       firstNonEmpty(cfg.TelnyxConnectionID, os.Getenv("TELNYX_CONNECTION_ID")),
			PhoneNumber:        cfg.PhoneNumber,
			PublicURL:          cfg.PublicURL,
			ListenAddr:         cfg.ListenAddr,
			Listener:           cfg.Listener,
			Mode:               gateway.PipelineModeAudio,
			MaxSessionDuration: cfg.MaxSessionDuration,
			Logger:             cfg.Logger,
		})
		if err != nil {
			return nil, fmt.Errorf("create telnyx audio gateway: %w", err)
		}
		return gw, nil
	default:
		return nil, fmt.Errorf("audio gateway: unsupported provider %q (supported: twilio, telnyx)", cfg.Provider)
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
