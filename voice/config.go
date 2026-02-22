// Package voice provides voice processing capabilities for omniagent.
package voice

// Config configures voice processing.
type Config struct {
	// Enabled indicates whether voice processing is enabled.
	Enabled bool
	// ResponseMode controls when to respond with voice: "auto", "always", "never".
	// "auto" responds with voice when the user sends a voice message.
	ResponseMode string
	// STT configures speech-to-text.
	STT STTConfig
	// TTS configures text-to-speech.
	TTS TTSConfig
}

// STTConfig configures the speech-to-text provider.
type STTConfig struct {
	// Provider is the STT provider name (e.g., "deepgram").
	Provider string
	// APIKey is the provider API key.
	APIKey string //nolint:gosec // G117: APIKey loaded from config file
	// Model is the provider-specific model identifier.
	Model string
	// Language is the BCP-47 language code. Empty for auto-detection.
	Language string
}

// TTSConfig configures the text-to-speech provider.
type TTSConfig struct {
	// Provider is the TTS provider name (e.g., "deepgram").
	Provider string
	// APIKey is the provider API key.
	APIKey string //nolint:gosec // G117: APIKey loaded from config file
	// Model is the provider-specific model identifier.
	Model string
	// VoiceID is the provider-specific voice identifier.
	VoiceID string
}
