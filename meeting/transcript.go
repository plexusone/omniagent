package meeting

import (
	"fmt"
	"strings"
	"time"
)

// Segment is one finalized stretch of transcribed speech.
type Segment struct {
	// Start and End are offsets from when transcription began.
	Start time.Duration `json:"start"`
	End   time.Duration `json:"end"`

	// Speaker is the speaker label, when the connection provides one.
	// Phone dial-in carries one mixed stream, so it is empty.
	Speaker string `json:"speaker,omitempty"`

	// Text is what was said.
	Text string `json:"text"`
}

// Transcript is the meeting's speech in order.
type Transcript []Segment

// String renders the transcript one segment per line with timestamps, for
// example "[00:01:05] Let's move the launch to Thursday."
func (t Transcript) String() string {
	var b strings.Builder
	for _, s := range t {
		b.WriteString("[" + clock(s.Start) + "] ")
		if s.Speaker != "" {
			b.WriteString(s.Speaker + ": ")
		}
		b.WriteString(s.Text)
		b.WriteString("\n")
	}
	return b.String()
}

// clock formats an offset as hh:mm:ss.
func clock(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	return fmt.Sprintf("%02d:%02d:%02d", h, m, d/time.Second)
}
