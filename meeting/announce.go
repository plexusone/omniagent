package meeting

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

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

// speak synthesizes text and plays it into the meeting, converting the
// synthesized audio to the meeting's format.
func speak(ctx context.Context, provider tts.Provider, config tts.SynthesisConfig,
	text string, out io.Writer, outFormat format.AudioFormat) error {
	result, err := provider.Synthesize(ctx, text, config)
	if err != nil {
		return fmt.Errorf("synthesize speech: %w", err)
	}
	from, err := synthesizedFormat(result)
	if err != nil {
		return err
	}
	audio := result.Audio
	if from != outFormat {
		if audio, err = converter.New().Convert(audio, from, outFormat); err != nil {
			return fmt.Errorf("convert speech from %s %d Hz: %w", from.Encoding, from.SampleRate, err)
		}
	}
	if _, err := out.Write(audio); err != nil {
		return fmt.Errorf("play speech: %w", err)
	}
	return nil
}

// synthesizedFormat reads the audio format of a TTS result. Providers name
// formats differently ("pcm", "linear16", "ulaw", "pcm_24000"); a numeric
// suffix gives the sample rate. Only raw PCM and mu-law can be played on a
// call; containers such as MP3 cannot.
func synthesizedFormat(result *tts.SynthesisResult) (format.AudioFormat, error) {
	name, rate := result.Format, result.SampleRate
	if i := strings.LastIndex(name, "_"); i > 0 {
		if n, err := strconv.Atoi(name[i+1:]); err == nil {
			name = name[:i]
			if rate == 0 {
				rate = n
			}
		}
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
