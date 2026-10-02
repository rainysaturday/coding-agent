package tui

import (
	"bufio"
	"io"
	"unicode"
	"unicode/utf8"
)

// KeyKind classifies a key event.
type KeyKind int

// Key kinds.
const (
	KeyRune KeyKind = iota
	KeyEnter
	KeyBackspace
	KeyDelete
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyHome
	KeyEnd
	KeyCtrlC
	KeyCtrlD
	KeyCtrlU
	KeyCtrlW
	KeyEscape
	KeyTab
)

// Key is a decoded input event.
type Key struct {
	Kind KeyKind
	Rune rune
}

// keyReader decodes terminal input (including escape sequences) into keys.
type keyReader struct {
	r *bufio.Reader
}

func newKeyReader(r io.Reader) *keyReader { return &keyReader{r: bufio.NewReader(r)} }

// Read decodes the next key.
func (k *keyReader) Read() (Key, error) {
	r, _, err := k.r.ReadRune()
	if err != nil {
		return Key{}, err
	}
	switch r {
	case utf8.RuneError:
		return Key{Kind: KeyEscape}, nil
	case '\r', '\n':
		return Key{Kind: KeyEnter}, nil
	case 0x7f, 0x08: // DEL / backspace
		return Key{Kind: KeyBackspace}, nil
	case 0x03:
		return Key{Kind: KeyCtrlC}, nil
	case 0x04:
		return Key{Kind: KeyCtrlD}, nil
	case 0x15:
		return Key{Kind: KeyCtrlU}, nil
	case 0x17:
		return Key{Kind: KeyCtrlW}, nil
	case '\t':
		return Key{Kind: KeyTab}, nil
	case 0x1b: // escape sequence
		return k.readEscape()
	default:
		if r < 0x20 {
			return Key{Kind: KeyEscape}, nil // ignore other control characters
		}
		if !unicode.IsPrint(r) && r != ' ' {
			return Key{Kind: KeyEscape}, nil
		}
		return Key{Kind: KeyRune, Rune: r}, nil
	}
}

// readEscape decodes an ANSI escape sequence.
func (k *keyReader) readEscape() (Key, error) {
	r, _, err := k.r.ReadRune()
	if err != nil {
		return Key{Kind: KeyEscape}, nil
	}
	if r != '[' && r != 'O' {
		return Key{Kind: KeyEscape}, nil
	}
	r2, _, err := k.r.ReadRune()
	if err != nil {
		return Key{Kind: KeyEscape}, nil
	}
	switch r2 {
	case 'A':
		return Key{Kind: KeyUp}, nil
	case 'B':
		return Key{Kind: KeyDown}, nil
	case 'C':
		return Key{Kind: KeyRight}, nil
	case 'D':
		return Key{Kind: KeyLeft}, nil
	case 'H':
		return Key{Kind: KeyHome}, nil
	case 'F':
		return Key{Kind: KeyEnd}, nil
	case '3': // ESC [ 3 ~ : delete
		_, _, _ = k.r.ReadRune()
		return Key{Kind: KeyDelete}, nil
	case '1', '4': // ESC [ 1 ~ / 4 ~ : home / end
		_, _, _ = k.r.ReadRune()
		if r2 == '1' {
			return Key{Kind: KeyHome}, nil
		}
		return Key{Kind: KeyEnd}, nil
	default:
		return Key{Kind: KeyEscape}, nil
	}
}
