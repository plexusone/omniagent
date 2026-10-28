package meeting

import (
	"strings"
	"testing"
	"time"

	"github.com/plexusone/omnimeet-core/attendee"
)

func TestRecordMarkdown(t *testing.T) {
	started := time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)
	full := (&Record{
		Target:     attendee.JoinTarget{Platform: attendee.PlatformGoogleMeet, JoinURL: "https://meet.google.com/abc-defg-hij"},
		Started:    started,
		Ended:      started.Add(45 * time.Minute),
		FinalState: attendee.StateEnded,
		Transcript: Transcript{{Start: time.Minute, Text: "Kickoff."}},
		Summary:    &Summary{Overview: "Planned the quarter.", Decisions: []string{"Ship v2"}},
	}).Markdown()
	for _, want := range []string{
		"# Meeting Notes",
		"- **Platform:** google_meet",
		"- **Meeting:** https://meet.google.com/abc-defg-hij",
		"- **Duration:** 45m0s",
		"- **Ended as:** ended",
		"## Summary\n\nPlanned the quarter.",
		"## Decisions\n\n- Ship v2",
		"## Transcript\n\n```text\n[00:01:00] Kickoff.\n```",
	} {
		if !strings.Contains(full, want) {
			t.Errorf("Markdown() missing %q:\n%s", want, full)
		}
	}

	empty := (&Record{}).Markdown()
	if !strings.Contains(empty, "Nothing was transcribed.") || strings.Contains(empty, "## Summary") ||
		strings.Contains(empty, "**Duration:**") || !strings.Contains(empty, "- **Platform:** unknown") {
		t.Errorf("Markdown() of an empty record:\n%s", empty)
	}
}
