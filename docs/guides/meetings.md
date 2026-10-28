# Attending Meetings by Phone

OmniAgent can attend Zoom, Google Meet, and Microsoft Teams meetings hosted by
others by dialing in on the phone, like a participant calling from a desk
phone. It announces itself, transcribes the meeting as it goes, answers when
someone addresses it by name, and writes meeting notes with a summary,
decisions, and action items when the meeting ends.

Phone dial-in works with any meeting whose invitation lists a dial-in number
and needs no app registration with the meeting platform. The agent hears one
mixed audio stream, so transcripts do not name speakers, and the agent appears
in the meeting as a phone number.

## Before You Start

You need:

- **A telephony account:** Twilio or Telnyx, with a phone number to call from.
- **A public URL** where the provider can reach OmniAgent's webhooks and media
  stream. Use `--ngrok` for local development.
- **Speech-to-text:** a streaming provider; Deepgram by default.
- **Text-to-speech** for the announcement and answers; ElevenLabs by default.
- **An LLM** for answers and the summary: Anthropic, OpenAI, or Gemini.

Set the credentials as environment variables:

```bash
export TWILIO_ACCOUNT_SID=AC...
export TWILIO_AUTH_TOKEN=...
export TWILIO_PHONE_NUMBER=+14155550199
export NGROK_AUTHTOKEN=...
export DEEPGRAM_API_KEY=...
export ELEVENLABS_API_KEY=...
export ANTHROPIC_API_KEY=...
```

For Telnyx, set `TELNYX_API_KEY`, `TELNYX_CONNECTION_ID`, and
`TELNYX_PHONE_NUMBER` instead, and pass `--provider telnyx`.

!!! warning "Consent"
    The agent announces that it is an AI assistant transcribing the meeting
    when it joins. Recording laws vary by jurisdiction; get consent from the
    meeting's participants before using it.

## Joining a Meeting

Save the meeting invitation as a text file or download the calendar event as
an `.ics` file, then:

```bash
omniagent meeting join --invite meeting.ics --ngrok
```

You can also pipe the invitation text:

```bash
pbpaste | omniagent meeting join --invite - --ngrok
```

OmniAgent reads the dial-in number, meeting ID, and passcode from the
invitation, calls a dial-in number for `--country` (US by default), and enters
the meeting ID and passcode on the keypad. It prints each transcribed line as
the meeting goes:

```text
Joining zoom meeting https://us02web.zoom.us/j/85012345678 by phone...
[00:00:12] Okay, let's get started with the launch review.
[00:00:19] Dana, where are we on the roadmap?
```

Press Ctrl+C to leave early. When the meeting ends or the agent leaves,
OmniAgent prints the summary and writes the meeting notes to
`meeting-<time>.md`:

```markdown
# Meeting Notes

- **Platform:** zoom
- **Duration:** 31m12s

## Summary

The team reviewed launch readiness and moved the launch to Thursday.

## Decisions

- Launch on Thursday

## Action Items

- Update the roadmap (owner: Dana)

## Transcript
...
```

## Asking the Agent a Question

After its announcement, which ends by inviting questions, the agent stays
silent until someone says its name:

```text
[00:14:02] OmniAgent, what did we decide about the launch?
[00:14:05] OmniAgent: We moved the launch to Thursday, and Dana is updating the roadmap.
```

Saying the name alone also works: after "Hey OmniAgent?", the next sentence is
taken as the question. The agent answers in a sentence or two from the recent
transcript and adds the exchange to the meeting notes under **Questions
Answered**. It answers one question at a time, and only after the meeting has
admitted it.

Speech to text may spell an unusual name differently. Spacing and case do not
matter ("Omni Agent" matches "OmniAgent"); for other spellings, add them with
`--answer-to`, for example `--answer-to Omni`.

!!! note "What participants say is untrusted"
    Anyone in the meeting can address the agent, so while answering it has no
    tools: it cannot send messages, look things up, or change anything. If
    asked to do something, it says it will include the request in the notes.

Pass `--no-answer` to keep the agent silent after its announcement.
`--no-announce` keeps it silent throughout.

## Options

| Flag | Default | Description |
|------|---------|-------------|
| `--invite` | | Invitation text or `.ics` file, or `-` for standard input (required) |
| `--provider` | `twilio` | Telephony provider: `twilio` or `telnyx` |
| `--phone` | `TWILIO_PHONE_NUMBER` or `TELNYX_PHONE_NUMBER` | Caller ID for the call |
| `--public-url` | | Public URL for the provider's webhooks |
| `--ngrok` | `false` | Open an ngrok tunnel instead of `--public-url` |
| `--listen` | `:8080` | Local address for the webhooks |
| `--country` | `US` | Country of the dial-in number to call |
| `--name` | `OmniAgent` | Name the agent announces itself with |
| `--on-behalf-of` | | Person the agent attends for, named in the announcement |
| `--no-announce` | `false` | Do not announce the agent; it then never speaks |
| `--no-answer` | `false` | Do not answer when addressed by name |
| `--answer-to` | | Other names the agent answers to, besides `--name` (repeatable) |
| `--stt`, `--stt-model`, `--language` | `deepgram` | Speech-to-text provider, model, and language |
| `--tts`, `--voice` | `elevenlabs` | Text-to-speech provider and voice for the announcement and answers |
| `--llm`, `--model` | detected | LLM provider and model for answers and the summary |
| `--no-summary` | `false` | Skip the summary |
| `--out` | `meeting-<time>.md` | Meeting notes file |
| `--json` | | Also write the full record as JSON |
| `--max-duration` | `2h` | Leave after this long |

## Limitations

- **No speaker names.** Phone dial-in delivers one mixed audio stream.
- **Answers start after the speaker finishes.** The agent answers once the
  question's sentence is transcribed, so expect a pause of a few seconds.
- **Admission is estimated.** The phone line does not report when the meeting
  admits the caller, so the agent announces itself once the keypad sequence has
  had time to finish. A waiting room is not detected.
- **Dial-in must be available.** Some meeting plans and regions have no phone
  dial-in; OmniAgent reports when an invitation lists none.
- **Phone menu prompts on Telnyx.** Telnyx streams audio from when the call is
  answered, so the meeting's phone menu prompts can appear at the start of the
  transcript.

## Using the Library

The command is built from reusable packages:

- `meeting.Attendant` (`github.com/plexusone/omniagent/meeting`) attends any
  OmniMeet `attendee.Session`: it announces, transcribes, answers when
  addressed, and summarizes. `meeting.LLMResponder` answers from the
  transcript with no tools; implement `meeting.Responder` to answer another
  way.
- `voice.NewAudioGateway` builds a Twilio or Telnyx gateway in audio mode.
- OmniMeet's `invite` and `dialin` packages parse invitations and join by
  phone; `dialin/dialintest` simulates meetings for tests.

```go
target, err := invite.ParseICS(icsFile)
if err != nil {
    return err
}
sess, err := dialin.NewJoiner(gw, dialin.WithCountry("US")).Join(ctx, target)
if err != nil {
    return err
}
record, err := meeting.New(meeting.Config{
    Name:        "OmniAgent",
    Transcriber: deepgram, // an omnivoice-core stt.StreamingProvider
    Speaker:     elevenlabs,
    Responder:   meeting.LLMResponder{Client: llm, Model: "claude-sonnet-5"},
    Summarizer:  meeting.LLMSummarizer{Client: llm, Model: "claude-sonnet-5"},
}).Attend(ctx, sess)
```
