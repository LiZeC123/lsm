package ui

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrInputClosed reports that the input stream ended before the dialogue was
// finished, which is what happens when lsm add runs without a terminal.
var ErrInputClosed = errors.New("input ended")

// Prompter asks the user questions and writes them to an output stream.
//
// Answers are never echoed: a real terminal already echoes what the user types,
// and echoing again would double the text.
type Prompter struct {
	in  *bufio.Scanner
	out io.Writer
}

// maxAnswerLen bounds a single answer. Command lines can be long, but not
// unboundedly so.
const maxAnswerLen = 1 << 20

// NewPrompter builds a prompter reading answers from in and writing questions
// to out.
func NewPrompter(in io.Reader, out io.Writer) *Prompter {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), maxAnswerLen)
	return &Prompter{in: scanner, out: out}
}

// Ask writes "<label>: " and returns the trimmed answer. ErrInputClosed is
// returned when the stream is exhausted.
func (p *Prompter) Ask(label string) (string, error) {
	if _, err := fmt.Fprintf(p.out, "%s: ", label); err != nil {
		return "", err
	}
	if !p.in.Scan() {
		if err := p.in.Err(); err != nil {
			return "", err
		}
		return "", ErrInputClosed
	}
	return strings.TrimSpace(p.in.Text()), nil
}

// AskValidated asks the question repeatedly until validate accepts an answer.
// validate returns either the value to use or the reason to ask again; that
// reason is shown to the user before the question is repeated.
func (p *Prompter) AskValidated(label string, validate func(string) (string, error)) (string, error) {
	for {
		answer, err := p.Ask(label)
		if err != nil {
			return "", err
		}
		value, err := validate(answer)
		if err == nil {
			return value, nil
		}
		if _, writeErr := fmt.Fprintf(p.out, "%s\n", err); writeErr != nil {
			return "", writeErr
		}
	}
}

// Confirm asks a yes/no question and defaults to "no". An exhausted input
// stream counts as declining.
func (p *Prompter) Confirm(question string) (bool, error) {
	answer, err := p.Ask(question + " [y/N]")
	if err != nil {
		if errors.Is(err, ErrInputClosed) {
			return false, nil
		}
		return false, err
	}
	switch strings.ToLower(answer) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
