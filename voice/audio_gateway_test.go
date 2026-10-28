package voice

import (
	"strings"
	"testing"

	"github.com/plexusone/omnivoice-core/gateway"
)

func TestNewAudioGatewayProviders(t *testing.T) {
	tests := []struct {
		name string
		cfg  AudioGatewayConfig
		want gateway.ProviderName
	}{
		{"twilio default", AudioGatewayConfig{
			TwilioAccountSID: "AC00000000000000000000000000000000", TwilioAuthToken: "token",
		}, gateway.ProviderTwilio},
		{"telnyx", AudioGatewayConfig{
			Provider: "telnyx", TelnyxAPIKey: "key", TelnyxConnectionID: "conn",
		}, gateway.ProviderTelnyx},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.PublicURL = "https://example.com"
			gw, err := NewAudioGateway(tt.cfg)
			if err != nil {
				t.Fatalf("NewAudioGateway() error: %v", err)
			}
			if gw.Name() != tt.want {
				t.Errorf("Name() = %q, want %q", gw.Name(), tt.want)
			}
			if _, ok := gw.(gateway.OptionCaller); !ok {
				t.Error("audio gateway does not accept call options, so keypad digits cannot be sent")
			}
		})
	}
}

func TestNewAudioGatewayErrors(t *testing.T) {
	if _, err := NewAudioGateway(AudioGatewayConfig{Provider: "twilio", TwilioAccountSID: "AC1", TwilioAuthToken: "t"}); err == nil ||
		!strings.Contains(err.Error(), "public URL") {
		t.Errorf("missing public URL error = %v", err)
	}
	if _, err := NewAudioGateway(AudioGatewayConfig{Provider: "vonage", PublicURL: "https://example.com"}); err == nil ||
		!strings.Contains(err.Error(), "unsupported provider") {
		t.Errorf("unsupported provider error = %v", err)
	}
	t.Setenv("TWILIO_ACCOUNT_SID", "")
	t.Setenv("TWILIO_AUTH_TOKEN", "")
	if _, err := NewAudioGateway(AudioGatewayConfig{PublicURL: "https://example.com"}); err == nil {
		t.Error("NewAudioGateway() without Twilio credentials returned nil error")
	}
}
