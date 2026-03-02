package voice

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/plexusone/omnivoice"
	_ "github.com/plexusone/omnivoice/providers/all" // Register all providers
)

// Processor handles voice transcription and synthesis using OmniVoice interfaces.
type Processor struct {
	sttProvider  omnivoice.STTProvider
	ttsProvider  omnivoice.TTSProvider
	config       Config
	logger       *slog.Logger
	responseMode string
}

// New creates a new voice processor with the configured providers.
func New(config Config, logger *slog.Logger) (*Processor, error) {
	if logger == nil {
		logger = slog.Default()
	}

	p := &Processor{
		config:       config,
		logger:       logger,
		responseMode: config.ResponseMode,
	}

	if p.responseMode == "" {
		p.responseMode = "auto"
	}

	// Initialize STT provider based on config
	if config.STT.Provider == "" {
		return nil, fmt.Errorf("STT provider not configured")
	}
	sttProv, err := omnivoice.GetSTTProvider(config.STT.Provider, omnivoice.WithAPIKey(config.STT.APIKey))
	if err != nil {
		return nil, fmt.Errorf("create %s stt: %w", config.STT.Provider, err)
	}
	p.sttProvider = sttProv

	// Initialize TTS provider based on config
	if config.TTS.Provider == "" {
		return nil, fmt.Errorf("TTS provider not configured")
	}
	ttsProv, err := omnivoice.GetTTSProvider(config.TTS.Provider, omnivoice.WithAPIKey(config.TTS.APIKey))
	if err != nil {
		return nil, fmt.Errorf("create %s tts: %w", config.TTS.Provider, err)
	}
	p.ttsProvider = ttsProv

	return p, nil
}

// TranscribeAudio converts audio to text using the configured STT provider.
func (p *Processor) TranscribeAudio(ctx context.Context, audio []byte, mimeType string) (string, error) {
	config := omnivoice.TranscriptionConfig{
		Model:    p.config.STT.Model,
		Language: p.config.STT.Language,
	}

	// Set encoding based on MIME type
	switch mimeType {
	case "audio/ogg; codecs=opus", "audio/ogg":
		config.Encoding = "opus"
	case "audio/mpeg", "audio/mp3":
		config.Encoding = "mp3"
	case "audio/wav", "audio/wave":
		config.Encoding = "wav"
	case "audio/flac":
		config.Encoding = "flac"
	}

	result, err := p.sttProvider.Transcribe(ctx, audio, config)
	if err != nil {
		return "", fmt.Errorf("transcribe: %w", err)
	}

	p.logger.Info("transcription complete",
		"provider", p.sttProvider.Name(),
		"text_length", len(result.Text),
		"language", result.Language)

	return result.Text, nil
}

// SynthesizeSpeech converts text to audio using the configured TTS provider.
// Returns audio bytes and MIME type.
func (p *Processor) SynthesizeSpeech(ctx context.Context, text string) ([]byte, string, error) {
	config := omnivoice.SynthesisConfig{
		VoiceID:      p.config.TTS.VoiceID,
		Model:        p.config.TTS.Model,
		OutputFormat: "mp3", // MP3 for broad compatibility; WhatsApp accepts this
	}

	result, err := p.ttsProvider.Synthesize(ctx, text, config)
	if err != nil {
		return nil, "", fmt.Errorf("synthesize: %w", err)
	}

	// Determine MIME type based on format
	mimeType := "audio/mpeg"
	switch result.Format {
	case "opus", "ogg":
		mimeType = "audio/ogg; codecs=opus"
	case "wav":
		mimeType = "audio/wav"
	case "mp3":
		mimeType = "audio/mpeg"
	}

	p.logger.Info("synthesis complete",
		"provider", p.ttsProvider.Name(),
		"audio_size", len(result.Audio),
		"format", result.Format)

	return result.Audio, mimeType, nil
}

// ResponseMode returns the voice response mode.
func (p *Processor) ResponseMode() string {
	return p.responseMode
}

// Close releases provider resources.
func (p *Processor) Close() error {
	return nil
}
