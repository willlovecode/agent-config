package plain

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/term"
)

func consoleKey(fd int, in *bufio.Reader) (answer string, err error) {
	state, err := term.MakeRaw(fd)
	if err != nil {
		return "", err
	}
	defer func() {
		if restoreErr := term.Restore(fd, state); restoreErr != nil {
			err = errors.Join(err, restoreErr)
		}
	}()
	return readKey(in)
}

func readKey(in *bufio.Reader) (string, error) {
	escaped, sequence := false, false
	for {
		key, err := in.ReadByte()
		if err != nil {
			return "", err
		}
		switch {
		case key == '\x03' || key == '\x04' || key == '\x1a':
			return "", io.EOF
		case key == '\x1b':
			escaped, sequence = true, false
			continue
		case sequence:
			// Skip terminal key sequences, including digits in modified arrow keys.
			sequence = key >= ' ' && key < '@'
			continue
		case escaped && (key == '[' || key == 'O'):
			escaped, sequence = false, true
			continue
		}
		escaped = false
		if key == '\r' || key == '\n' {
			return "", nil
		}
		if key >= '!' && key <= '~' {
			return string(key), nil
		}
	}
}

func (s *session) ask(prompt string) (string, error) {
	return s.readAnswer(prompt, s.key)
}

// readAnswer uses line input for pipes, --plain and free-text search.
func (s *session) readAnswer(prompt string, key func() (string, error)) (string, error) {
	if prompt != "" {
		s.lines(prompt)
	}
	const hints = "(b) back  (q) quit"
	if s.terminal {
		fmt.Fprint(s.out, hints+" > ")
	} else {
		s.lines(hints)
	}
	var answer string
	var err error
	if key != nil {
		answer, err = key()
		fmt.Fprintln(s.out, answer)
	} else {
		answer, err = s.in.ReadString('\n')
		if err == io.EOF && answer != "" {
			err = nil
		}
	}
	if err == io.EOF {
		if s.terminal && key == nil {
			fmt.Fprintln(s.out)
		}
		return "", errQuit
	}
	if err != nil {
		return "", err
	}
	answer = strings.TrimSpace(answer)
	switch strings.ToLower(answer) {
	case "b", "<":
		return "", errBack
	case "q":
		return "", errQuit
	}
	return answer, nil
}

func (s *session) yesNo(prompt string, yesDefault bool) (bool, error) {
	choices := " [y/N]"
	if yesDefault {
		choices = " [Y/n]"
	}
	for {
		answer, err := s.ask(prompt + choices)
		if err != nil {
			return false, err
		}
		switch strings.ToLower(answer) {
		case "":
			return yesDefault, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		}
		s.lines("Choose y or n.")
	}
}

func (s *session) waitForEnter(prompt string) error {
	for {
		answer, err := s.ask(prompt)
		if err != nil || answer == "" {
			return err
		}
	}
}
