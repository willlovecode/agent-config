package screen

import "bufio"

// Key is one keypress: a printable ASCII character, or one of the keys below.
type Key rune

const (
	KeyEnter Key = '\r'
	KeyCtrlC Key = 3  // also Ctrl-D and Ctrl-Z: leave without writing anything
	KeyEOF   Key = -1 // the input ended
)

// readKey reads one keypress from raw bytes. Escape sequences (arrow keys, function
// keys) and bytes outside ASCII are skipped without a trace.
func readKey(r *bufio.Reader) (Key, error) {
	for {
		b, err := r.ReadByte()
		if err != nil {
			return KeyEOF, err
		}
		switch {
		case b == 0x03 || b == 0x04 || b == 0x1a:
			return KeyCtrlC, nil
		case b == 0x1b:
			skipEscape(r)
		case b == '\r' || b == '\n':
			return KeyEnter, nil
		case b > ' ' && b <= '~':
			return Key(b), nil
		}
	}
}

// skipEscape drops the rest of a CSI or SS3 sequence. A lone Escape is not followed by
// buffered bytes, and is dropped without waiting.
func skipEscape(r *bufio.Reader) {
	if r.Buffered() == 0 {
		return
	}
	if next, _ := r.Peek(1); next[0] != '[' && next[0] != 'O' {
		return
	}
	r.ReadByte()
	for {
		b, err := r.ReadByte()
		if err != nil || (b >= 0x40 && b <= 0x7e) {
			return
		}
	}
}
