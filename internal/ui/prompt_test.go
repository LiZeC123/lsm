package ui

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestAskWritesTheLabelAndTrimsTheAnswer(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader("  web  \n"), &out)

	got, err := p.Ask("Service name")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if got != "web" {
		t.Fatalf("answer = %q, want %q", got, "web")
	}
	if out.String() != "Service name: " {
		t.Fatalf("prompt = %q", out.String())
	}
}

func TestAskReadsEveryLineInOrder(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader("one\ntwo\n"), &out)

	for _, want := range []string{"one", "two"} {
		got, err := p.Ask("Question")
		if err != nil {
			t.Fatalf("Ask: %v", err)
		}
		if got != want {
			t.Fatalf("answer = %q, want %q", got, want)
		}
	}
}

func TestAskReportsClosedInput(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader(""), &out)

	if _, err := p.Ask("Question"); !errors.Is(err, ErrInputClosed) {
		t.Fatalf("Ask = %v, want ErrInputClosed", err)
	}
}

func TestAskValidatedRepeatsUntilTheAnswerIsAccepted(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader("bad name!\nweb\n"), &out)

	attempts := 0
	got, err := p.AskValidated("Service name", func(answer string) (string, error) {
		attempts++
		if strings.Contains(answer, " ") {
			return "", fmt.Errorf("invalid service name '%s'", answer)
		}
		return answer, nil
	})
	if err != nil {
		t.Fatalf("AskValidated: %v", err)
	}
	if got != "web" {
		t.Fatalf("answer = %q, want web", got)
	}
	if attempts != 2 {
		t.Fatalf("validate ran %d times, want 2", attempts)
	}
	if !strings.Contains(out.String(), "invalid service name 'bad name!'") {
		t.Fatalf("the rejection reason must be shown, got %q", out.String())
	}
	if strings.Count(out.String(), "Service name: ") != 2 {
		t.Fatalf("the question must be repeated, got %q", out.String())
	}
}

func TestAskValidatedPropagatesClosedInput(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader(""), &out)

	_, err := p.AskValidated("Service name", func(string) (string, error) {
		return "", errors.New("never")
	})
	if !errors.Is(err, ErrInputClosed) {
		t.Fatalf("AskValidated = %v, want ErrInputClosed", err)
	}
}

func TestConfirm(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"Y\n", true},
		{"yes\n", true},
		{"YES\n", true},
		{" y \n", true},
		{"n\n", false},
		{"\n", false}, // bare Enter takes the default
		{"no\n", false},
		{"maybe\n", false},
		{"", false}, // closed input counts as declining
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%q", tc.input), func(t *testing.T) {
			var out bytes.Buffer
			p := NewPrompter(strings.NewReader(tc.input), &out)

			got, err := p.Confirm("Delete service 'web'?")
			if err != nil {
				t.Fatalf("Confirm: %v", err)
			}
			if got != tc.want {
				t.Fatalf("Confirm = %v, want %v", got, tc.want)
			}
			if want := "Delete service 'web'? [y/N]: "; out.String() != want {
				t.Fatalf("prompt = %q, want %q", out.String(), want)
			}
		})
	}
}

func TestAskAcceptsLongAnswers(t *testing.T) {
	long := strings.Repeat("a", 100_000)
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader(long+"\n"), &out)

	got, err := p.Ask("Command")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if got != long {
		t.Fatalf("answer was truncated to %d characters", len(got))
	}
}
