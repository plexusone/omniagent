package meeting

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/plexusone/omnivoice-core/audio/converter"
	"github.com/plexusone/omnivoice-core/audio/format"
	"github.com/plexusone/omnivoice-core/tts"
)

// DefaultAnnouncement returns the disclosure spoken when the agent joins: who
// it is, whom it attends for, and that it is transcribing.
func DefaultAnnouncement(name, onBehalfOf string) string {
	who := "an AI assistant"
	if onBehalfOf != "" {
		who += " attending for " + onBehalfOf
	}
	return fmt.Sprintf("Hi, this is %s, %s. I'm transcribing this meeting to take notes "+
		"and will share a summary afterwards.", name, who)
}

// answerInvitation is added to the default announcement when the agent
// answers questions.
const answerInvitation = " Say my name if you'd like me to answer a question."

// speak synthesizes text and plays it into the meeting, converting the
// synthesized audio to the meeting's format. It returns how long the speech
// lasts: the write returns once the audio is queued, before it is heard.
func speak(ctx context.Context, provider tts.Provider, config tts.SynthesisConfig,
	text string, out io.Writer, outFormat format.AudioFormat) (time.Duration, error) {
	result, err := provider.Synthesize(ctx, text, config)
	if err != nil {
		return 0, fmt.Errorf("synthesize speech: %w", err)
	}
	from, err := synthesizedFormat(result, config)
	if err != nil {
		return 0, err
	}
	audio := result.Audio
	if from != outFormat {
		if audio, err = converter.New().Convert(audio, from, outFormat); err != nil {
			return 0, fmt.Errorf("convert speech from %s %d Hz: %w", from.Encoding, from.SampleRate, err)
		}
	}
	if _, err := out.Write(audio); err != nil {
		return 0, fmt.Errorf("play speech: %w", err)
	}
	return playTime(len(audio), outFormat), nil
}

// playTime is how long n bytes of audio in f take to play.
func playTime(n int, f format.AudioFormat) time.Duration {
	bytesPerSample := 2
	if format.Encoding(f.Encoding).Normalize() == format.MuLaw {
		bytesPerSample = 1
	}
	perSecond := f.SampleRate * max(f.Channels, 1) * bytesPerSample
	if perSecond == 0 {
		return 0
	}
	return time.Duration(n) * time.Second / time.Duration(perSecond)
}

// synthesizedFormat reads the audio format of a TTS result. Providers name
// formats differently ("pcm", "linear16", "ulaw", "pcm_24000"). The sample
// rate comes from a numeric suffix on the name, then from the result, then
// from the request, since some providers report no rate or a default one.
// Only raw PCM and mu-law can be played on a call; containers such as MP3
// cannot.
func synthesizedFormat(result *tts.SynthesisResult, requested tts.SynthesisConfig) (format.AudioFormat, error) {
	name := result.Format
	rate := 0
	if i := strings.LastIndex(name, "_"); i > 0 {
		if n, err := strconv.Atoi(name[i+1:]); err == nil {
			name, rate = name[:i], n
		}
	}
	if rate == 0 {
		rate = result.SampleRate
	}
	if rate == 0 {
		rate = requested.SampleRate
	}
	enc := format.Encoding(name).Normalize()
	switch enc {
	case format.Linear16, format.MuLaw:
	default:
		return format.AudioFormat{}, fmt.Errorf("speech format %q cannot be played on a call; request raw PCM or mu-law", result.Format)
	}
	if rate == 0 {
		if enc != format.MuLaw {
			return format.AudioFormat{}, fmt.Errorf("speech format %q has no sample rate", result.Format)
		}
		rate = 8000
	}
	return format.AudioFormat{Encoding: string(enc), SampleRate: rate, Channels: 1}, nil
}
