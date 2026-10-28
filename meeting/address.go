package meeting

import (
	"strings"
	"unicode"
)

// maxNameWords is the most words a name may be split into by speech to
// text, as in "Omni Agent" for "OmniAgent".
const maxNameWords = 4

// fillerWords may surround the agent's name in an utterance that only gets
// its attention, such as "Hey OmniAgent?".
var fillerWords = map[string]bool{
	"hey": true, "hi": true, "hello": true, "ok": true, "okay": true,
	"so": true, "um": true, "uh": true, "yo": true, "and": true,
}

// addressed reports whether text names the agent by one of names. Case,
// punctuation, and spacing are ignored, so "omni agent," matches
// "OmniAgent". nameOnly reports that the utterance says nothing besides the
// name and filler words, so the question is still to come.
func addressed(text string, names []string) (ok, nameOnly bool) {
	words := splitWords(text)
	for _, name := range names {
		key := strings.Join(splitWords(name), "")
		if key == "" {
			continue
		}
		for i := range words {
			// Walk words from i while they spell out the start of key.
			pos := 0
			for j := i; j < len(words) && j < i+maxNameWords; j++ {
				if !strings.HasPrefix(key[pos:], words[j]) {
					break
				}
				pos += len(words[j])
				if pos == len(key) {
					return true, onlyFiller(words[:i]) && onlyFiller(words[j+1:])
				}
			}
		}
	}
	return false, false
}

// splitWords lowercases text and splits it into runs of letters and digits.
func splitWords(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func onlyFiller(words []string) bool {
	for _, w := range words {
		if !fillerWords[w] {
			return false
		}
	}
	return true
}
