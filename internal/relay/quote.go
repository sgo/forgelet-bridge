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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-23T13:23:32+02:00","module_hash":"003fe037ca4146c8cd76c77358f722ec87dcfd0bf819bff7ca718d21f8ab0b8a","functions":[{"id":"func/OwnWords","name":"OwnWords","line":10,"end_line":27,"hash":"31cd39627783eecb08354b90202a754a28aca2ed0a2617e2b7806280213d5434"},{"id":"func/quoteLine","name":"quoteLine","line":31,"end_line":36,"hash":"cc1dc1fe95a4fc6e70789429ba606fbd220795aab6f696b002afe9c72410fd99"},{"id":"func/normalized","name":"normalized","line":39,"end_line":41,"hash":"097d5fff55eba5993d7676fdf87979ffc6e9bf9055235f997914acff673e77af"},{"id":"func/repliedTo","name":"repliedTo","line":45,"end_line":50,"hash":"a811cffb639aa6f32bac3df8df95d8d1cb08ac075ed0918a8bec95d787272f8e"}]}
// mutate4go-manifest-end
