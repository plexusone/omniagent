# Release Notes: v0.2.0

**Release Date:** 2026-02-22

## Summary

This release adds voice note support for WhatsApp, enabling OmniAgent to transcribe incoming voice messages and respond with synthesized speech.

## Highlights

- **Voice note support** via OmniVoice integration (Deepgram provider)

## New Features

### Voice Processing

New `voice/` package that integrates with OmniVoice for speech-to-text and text-to-speech:

- **Transcription**: Incoming voice notes are automatically transcribed using Deepgram's nova-2 model
- **Synthesis**: Agent responses are synthesized to voice using Deepgram's Aura voices
- **Smart responses**: When responses contain URLs, both voice and text are sent so users can click links

### Configuration

Voice processing can be enabled via configuration:

```yaml
voice:
  enabled: true
  response_mode: auto  # auto, always, never
  stt:
    provider: deepgram
    model: nova-2
  tts:
    provider: deepgram
    model: aura-asteria-en
    voice_id: aura-asteria-en
```

Or via environment variables:

```bash
export DEEPGRAM_API_KEY="your-key"
export OMNIAGENT_VOICE_ENABLED=true
```

### Response Modes

- `auto` (default): Respond with voice only when the user sends a voice message
- `always`: Always respond with voice
- `never`: Never respond with voice (text only)

## Usage

1. Get a Deepgram API key from https://deepgram.com
2. Enable voice in your configuration
3. Run the gateway
4. Send a voice note via WhatsApp - you'll get a voice response!

```bash
export DEEPGRAM_API_KEY="your-key"
export OPENAI_API_KEY="your-key"
export WHATSAPP_ENABLED=true
export OMNIAGENT_VOICE_ENABLED=true
omniagent gateway run
```

## Dependencies

- Added `github.com/agentplexus/omnivoice` v0.4.3
- Added `github.com/agentplexus/omnivoice-deepgram` v0.3.1
- Upgraded `github.com/agentplexus/omnichat` to v0.2.0

## Upgrade Guide

This release is backwards compatible. Voice processing is disabled by default.

```bash
go get github.com/agentplexus/omniagent@v0.2.0
```

To enable voice, set `OMNIAGENT_VOICE_ENABLED=true` and provide a `DEEPGRAM_API_KEY`.
