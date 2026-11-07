package screen

import (
	"bufio"
	"strings"
	"testing"
)

func TestWrap(t *testing.T) {
	for _, tc := range []struct {
		in   string
		w, h int
		want []string
	}{
		{"one two three four", 9, 0, []string{"one two", "three", "four"}},
		{"  indented words go on", 12, 2, []string{"  indented", "    words go", "    on"}},
		{"", 10, 0, []string{""}},
		{"aaaaaaaaaaaaaaaaaaaa", 8, 0, []string{"aaaaaaaa", "aaaaaaaa", "aaaa"}},
		{"tab\tand\nnewline", 40, 0, []string{"tab and newline"}},
	} {
		got := wrap(tc.in, tc.w, tc.h)
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("wrap(%q, %d, %d) = %q, want %q", tc.in, tc.w, tc.h, got, tc.want)
		}
	}
}

func TestTruncateCenterAndMarkers(t *testing.T) {
	if got := truncate("abcdefghij", 8); got != "abcde..." {
		t.Errorf("truncate: %q", got)
	}
	if got := truncate("short", 8); got != "short" {
		t.Errorf("truncate: %q", got)
	}
	if got := center("ab", 6); got != "  ab" {
		t.Errorf("center: %q", got)
	}
	styled := bold("ab") + dim("cd")
	if width(styled) != 4 || sgr(styled, false) != "abcd" || sgr(styled, true) != "\x1b[1mab\x1b[0m\x1b[2mcd\x1b[0m" {
		t.Errorf("markers: width %d, plain %q, styled %q", width(styled), sgr(styled, false), sgr(styled, true))
	}
}

func TestReadKey(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []Key
	}{
		{"y", []Key{'y'}},
		{"ab\r", []Key{'a', 'b', KeyEnter}},
		{"1\n", []Key{'1', KeyEnter}},
		{"\x03", []Key{KeyCtrlC}},
		{"\x04", []Key{KeyCtrlC}},
		{"\x1a", []Key{KeyCtrlC}},
		// arrow keys, modified arrow keys, function keys and a lone Escape leave no key behind
		{"\x1b[Aq", []Key{'q'}},
		{"\x1b[1;2Bq", []Key{'q'}},
		{"\x1bOPq", []Key{'q'}},
		{"\x1b[15~q", []Key{'q'}},
		{"\x1b", nil},
		{"\x1bq", []Key{'q'}},
		{" \t\xc3\xa4x", []Key{'x'}},
	} {
		r := bufio.NewReader(strings.NewReader(tc.in))
		var got []Key
		for {
			k, err := readKey(r)
			if err != nil {
				if k != KeyEOF {
					t.Errorf("%q: error without KeyEOF", tc.in)
				}
				break
			}
			got = append(got, k)
		}
		if len(got) != len(tc.want) {
			t.Errorf("%q: got %q, want %q", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%q: got %q, want %q", tc.in, got, tc.want)
			}
		}
	}
}
