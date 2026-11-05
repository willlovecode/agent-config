package tui

// The pixel art: a block-letter logo and a small mascot whose face follows what the
// installer is doing. Each piece has an ASCII twin for terminals without UTF-8.

var logo = []string{
	"▄▀█ █▀▀ ▀█▀ █▄▀",
	"█▀█ █▄█  █  █ █",
}

var logoASCII = []string{
	` /\  /~ ~|~ |/`,
	`/~~\ \_| |  |\`,
}

// mood picks the mascot's face.
type mood int

const (
	moodHi mood = iota
	moodThink
	moodWork
	moodHappy
	moodOops
)

var eyes = map[mood][2]string{
	moodHi:    {"• •", "o o"},
	moodThink: {"• ‿", "o ."},
	moodWork:  {"- -", "- -"},
	moodHappy: {"^ ^", "^ ^"},
	moodOops:  {"x x", "x x"},
}

// mascot returns three rows; blink closes the eyes for one frame.
func mascot(m mood, ascii, blink bool) []string {
	i := 0
	if ascii {
		i = 1
	}
	e := eyes[m][i]
	if blink && m != moodOops {
		e = eyes[moodWork][i]
	}
	if ascii {
		return []string{" .---. ", "( " + e + " )", " '---' "}
	}
	return []string{"▄█▀▀▀█▄", "█ " + e + " █", "▀█▄▄▄█▀"}
}

// Pixel spinner frames, and the ASCII ones DOS installers drew.
var spinPixel = []string{"▖", "▘", "▝", "▗"}
var spinASCII = []string{"|", "/", "-", `\`}

// glyphs used by the layout, in Unicode and ASCII.
type glyphs struct {
	done, current, todo, rail, cursor, on, off, radioOn, radioOff, pipOn, pipOff, heart string
	tl, tr, bl, br, h, v                                                                string
}

var unicodeGlyphs = glyphs{
	done: "◇", current: "◆", todo: "○", rail: "│", cursor: "▸",
	on: "[x]", off: "[ ]", radioOn: "●", radioOff: "○", pipOn: "▰", pipOff: "▱", heart: "♥",
	tl: "╭", tr: "╮", bl: "╰", br: "╯", h: "─", v: "│",
}

var asciiGlyphs = glyphs{
	done: "o", current: "*", todo: ".", rail: "|", cursor: ">",
	on: "[x]", off: "[ ]", radioOn: "(*)", radioOff: "( )", pipOn: "#", pipOff: "-", heart: "<3",
	tl: "+", tr: "+", bl: "+", br: "+", h: "-", v: "|",
}
