package relay

import "testing"

func TestOwnWordsStripsTheMessageThePhoneQuoted(t *testing.T) {
	body := "> is the build green?\n\nyes, the build is green"
	if got := OwnWords(body); got != "yes, the build is green" {
		t.Fatalf("OwnWords(%q) = %q, want the operator's own words", body, got)
	}
}

func TestOwnWordsStripsAQuoteOfMoreThanOneLine(t *testing.T) {
	body := "> Approval for phone-approvals in forgelet-bridge\n> Gate: coder → refactorer\n> \n> React ✅ to approve.\n\nrefund figures do not add up"
	if got := OwnWords(body); got != "refund figures do not add up" {
		t.Fatalf("OwnWords = %q, want only the words after the quote", got)
	}
}

func TestOwnWordsLeavesAMessageThatQuotesNothingAlone(t *testing.T) {
	body := "the timesheet total is still wrong"
	if got := OwnWords(body); got != body {
		t.Fatalf("OwnWords(%q) = %q, want the message unchanged", body, got)
	}
}

func TestOwnWordsKeepsTheBlankLinesInsideTheWords(t *testing.T) {
	body := "> is the build green?\n\nfirst thought\n\nsecond thought"
	if got := OwnWords(body); got != "first thought\n\nsecond thought" {
		t.Fatalf("OwnWords = %q, want the words with their blank line", got)
	}
}

func TestOwnWordsOfAQuoteWithNothingSaidIsEmpty(t *testing.T) {
	body := "> is the build green?\n"
	if got := OwnWords(body); got != "" {
		t.Fatalf("OwnWords(%q) = %q, want nothing said", body, got)
	}
}
