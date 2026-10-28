package meeting

import (
	"fmt"
	"strings"
	"time"
)

// Markdown renders the record as a meeting notes document: details, the
// summary when there is one, and the transcript.
func (r *Record) Markdown() string {
	var b strings.Builder
	b.WriteString("# Meeting Notes\n\n")

	platform := r.Target.Platform.String()
	fmt.Fprintf(&b, "- **Platform:** %s\n", platform)
	if r.Target.JoinURL != "" {
		fmt.Fprintf(&b, "- **Meeting:** %s\n", r.Target.JoinURL)
	}
	if !r.Started.IsZero() {
		fmt.Fprintf(&b, "- **Started:** %s\n", r.Started.Format(time.RFC1123))
	}
	if !r.Ended.IsZero() && !r.Started.IsZero() {
		fmt.Fprintf(&b, "- **Duration:** %s\n", r.Ended.Sub(r.Started).Round(time.Second))
	}
	if r.FinalState != "" {
		fmt.Fprintf(&b, "- **Ended as:** %s\n", r.FinalState)
	}
	b.WriteString("\n")

	if r.Summary != nil {
		b.WriteString(r.Summary.Markdown())
		b.WriteString("\n")
	}

	b.WriteString("## Transcript\n\n")
	if len(r.Transcript) == 0 {
		b.WriteString("Nothing was transcribed.\n")
		return b.String()
	}
	b.WriteString("```text\n")
	b.WriteString(r.Transcript.String())
	b.WriteString("```\n")
	return b.String()
}
