package voice

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/agentplexus/omnivoice/stt"
	"github.com/agentplexus/omnivoice/tts"

	deepgramstt "github.com/agentplexus/omnivoice-deepgram/omnivoice/stt"
	deepgramtts "github.com/agentplexus/omnivoice-deepgram/omnivoice/tts"
)

// Processor handles voice transcription and synthesis using OmniVoice interfaces.
type Processor struct {
	sttProvider  stt.Provider
	ttsProvider  tts.Provider
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
	switch config.STT.Provider {
	case "deepgram":
		sttProv, err := deepgramstt.New(deepgramstt.WithAPIKey(config.STT.APIKey))
		if err != nil {
			return nil, fmt.Errorf("create deepgram stt: %w", err)
		}
		p.sttProvider = sttProv
	case "":
		return nil, fmt.Errorf("STT provider not configured")
	default:
		return nil, fmt.Errorf("unsupported STT provider: %s", config.STT.Provider)
	}

	// Initialize TTS provider based on config
	switch config.TTS.Provider {
	case "deepgram":
		ttsProv, err := deepgramtts.New(deepgramtts.WithAPIKey(config.TTS.APIKey))
		if err != nil {
			return nil, fmt.Errorf("create deepgram tts: %w", err)
		}
		p.ttsProvider = ttsProv
	case "":
		return nil, fmt.Errorf("TTS provider not configured")
	default:
		return nil, fmt.Errorf("unsupported TTS provider: %s", config.TTS.Provider)
	}

	return p, nil
}

// TranscribeAudio converts audio to text using the configured STT provider.
func (p *Processor) TranscribeAudio(ctx context.Context, audio []byte, mimeType string) (string, error) {
	config := stt.TranscriptionConfig{
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
	config := tts.SynthesisConfig{
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
