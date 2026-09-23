package relay

import "strings"

// OwnWords is what the operator actually said in a message their phone sent by
// quoting another one. The phone writes the message it quotes into the body,
// each of its lines marked the way Matrix clients mark a quote, and that quote
// comes out again here before the words are read: an answer is what the
// operator said, not the message they answered.
func OwnWords(body string) string {
	quoted := true
	var words []string
	for _, line := range strings.Split(normalized(body), "\n") {
		if quoted {
			if _, isQuote := quoteLine(line); isQuote {
				continue
			}
			// The phone separates the quote from the words with a blank line.
			if strings.TrimSpace(line) == "" {
				continue
			}
			quoted = false
		}
		words = append(words, line)
	}
	return strings.TrimSpace(strings.Join(words, "\n"))
}

// quoteLine reports whether a line is part of the quoted message, and what the
// line says without the marker the phone put in front of it.
func quoteLine(line string) (string, bool) {
	if !strings.HasPrefix(line, ">") {
		return "", false
	}
	return strings.TrimPrefix(strings.TrimPrefix(line, ">"), " "), true
}

// normalized is a body with the line endings of every client spelled one way.
func normalized(body string) string {
	return strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\r", "\n")
}

// repliedTo is the message a reply answers: the thread it was written in, or
// the message it quoted, which is the reply a phone sends.
func repliedTo(reply RoomEvent) string {
	if reply.ThreadRoot != "" {
		return reply.ThreadRoot
	}
	return reply.ReplyTo
}
