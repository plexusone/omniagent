# WhatsApp Setup

This guide covers setting up OmniAgent with WhatsApp.

## Overview

OmniAgent connects to WhatsApp using the WhatsApp Web protocol via [whatsmeow](https://github.com/tulir/whatsmeow). This allows your agent to send and receive messages without requiring WhatsApp Business API access.

## Quick Start

```bash
export OPENAI_API_KEY="sk-..."
WHATSAPP_ENABLED=true omniagent gateway run
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `WHATSAPP_ENABLED` | Enable WhatsApp channel | `false` |
| `WHATSAPP_DB_PATH` | Session database path | `whatsapp.db` |

### Config File

```yaml
channels:
  whatsapp:
    enabled: true
    db_path: "whatsapp.db"
```

## Linking Your Phone

When you start the gateway with WhatsApp enabled:

1. A QR code appears in your terminal
2. Open WhatsApp on your phone
3. Go to **Settings** → **Linked Devices** → **Link a Device**
4. Scan the QR code

!!! note "Session Persistence"
    The session is stored in `whatsapp.db`. Delete this file to unlink and start fresh.

## How It Works

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│  WhatsApp   │───▶│  OmniAgent  │───▶│    LLM      │
│   (Phone)   │◀───│   Gateway   │◀───│  Provider   │
└─────────────┘    └─────────────┘    └─────────────┘
```

1. Messages arrive via WhatsApp Web protocol
2. OmniAgent processes them through the AI agent
3. Responses are sent back to the conversation

## Voice Notes

OmniAgent supports voice notes when voice processing is enabled:

```yaml
channels:
  whatsapp:
    enabled: true

voice:
  enabled: true
  response_mode: auto  # Reply with voice to voice messages
  stt:
    provider: deepgram
  tts:
    provider: deepgram
```

See [Voice Integration](voice.md) for details.

## Troubleshooting

### QR Code Not Appearing

Ensure no other WhatsApp Web sessions are interfering. Delete `whatsapp.db` and restart.

### Connection Drops

The WhatsApp Web connection may drop occasionally. OmniAgent will automatically reconnect when possible.

### Rate Limiting

WhatsApp may rate-limit accounts sending too many messages. Use reasonable response times and avoid spamming.

## Security Considerations

- The `whatsapp.db` file contains your session credentials - keep it secure
- Messages are processed through your configured LLM provider
- Consider the privacy implications of routing messages through AI
