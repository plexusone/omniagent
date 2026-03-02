# Voice Integration

OmniAgent supports voice notes via [OmniVoice](https://github.com/plexusone/omnivoice), providing speech-to-text (STT) and text-to-speech (TTS) capabilities.

## Overview

When voice processing is enabled:

1. Incoming voice messages are transcribed to text
2. The text is processed by the AI agent
3. Responses can be synthesized back to speech

## Supported Providers

| Provider | STT | TTS | Notes |
|----------|-----|-----|-------|
| Deepgram | ✅ | ✅ | Nova-2 for STT, Aura voices for TTS |
| OpenAI | ✅ | ✅ | Whisper for STT, TTS-1 for TTS |
| ElevenLabs | ❌ | ✅ | High-quality voice synthesis |

## Configuration

### Environment Variables

| Variable | Description |
|----------|-------------|
| `DEEPGRAM_API_KEY` | Deepgram API key |
| `OPENAI_API_KEY` | OpenAI API key (for Whisper/TTS) |
| `ELEVENLABS_API_KEY` | ElevenLabs API key |
| `OMNIAGENT_VOICE_ENABLED` | Enable voice processing |
| `OMNIAGENT_VOICE_RESPONSE_MODE` | Response mode: `auto`, `always`, `never` |

### Config File

```yaml
voice:
  enabled: true
  response_mode: auto

  stt:
    provider: deepgram
    model: nova-2

  tts:
    provider: deepgram
    model: aura-asteria-en
    voice_id: aura-asteria-en
```

## Response Modes

| Mode | Behavior |
|------|----------|
| `auto` | Reply with voice only to voice messages |
| `always` | Always reply with voice |
| `never` | Never reply with voice (text only) |

## Provider Setup

### Deepgram

1. Sign up at [deepgram.com](https://deepgram.com)
2. Create an API key
3. Set `DEEPGRAM_API_KEY` environment variable

```yaml
voice:
  stt:
    provider: deepgram
    model: nova-2
  tts:
    provider: deepgram
    model: aura-asteria-en
```

### OpenAI

Uses your existing OpenAI API key:

```yaml
voice:
  stt:
    provider: openai
    model: whisper-1
  tts:
    provider: openai
    model: tts-1
    voice_id: alloy
```

### ElevenLabs (TTS only)

1. Sign up at [elevenlabs.io](https://elevenlabs.io)
2. Create an API key
3. Set `ELEVENLABS_API_KEY` environment variable

```yaml
voice:
  tts:
    provider: elevenlabs
    voice_id: your-voice-id
```

## Architecture

OmniVoice uses a provider registry pattern:

```go
import (
    "github.com/plexusone/omnivoice"
    _ "github.com/plexusone/omnivoice/providers/all" // Register all providers
)

// Get providers by name
stt, _ := omnivoice.GetSTTProvider("deepgram", omnivoice.WithAPIKey(key))
tts, _ := omnivoice.GetTTSProvider("elevenlabs", omnivoice.WithAPIKey(key))
```

## Troubleshooting

### Voice Not Working

1. Verify API keys are set correctly
2. Check that voice is enabled in config
3. Ensure the provider supports your chosen model

### Poor Transcription Quality

- Use Deepgram Nova-2 or OpenAI Whisper for best results
- Ensure audio quality is reasonable
- Check language settings match the spoken language

### TTS Sounds Robotic

- Try different voice IDs
- ElevenLabs offers the most natural-sounding voices
- Adjust model settings if available
