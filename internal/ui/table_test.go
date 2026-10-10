package ui

import (
	"bytes"
	"testing"
)

func TestTableAlignsColumns(t *testing.T) {
	var buf bytes.Buffer
	table := NewTable(&buf, "NAME", "STATUS", "PID")
	table.Row("web", "running", "12345")
	table.Row("worker", "stopped", "-")
	if err := table.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	want := "NAME    STATUS   PID\n" +
		"web     running  12345\n" +
		"worker  stopped  -\n"
	if got := buf.String(); got != want {
		t.Fatalf("table mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestTableWithoutRowsPrintsOnlyTheHeader(t *testing.T) {
	var buf bytes.Buffer
	if err := NewTable(&buf, "NAME", "COMMAND", "WORK_DIR").Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got, want := buf.String(), "NAME  COMMAND  WORK_DIR\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// A command may contain tabs or newlines (YAML block scalars); they must not be
// able to break the alignment of the table.
func TestTableEscapesControlCharacters(t *testing.T) {
	var buf bytes.Buffer
	table := NewTable(&buf, "NAME", "COMMAND")
	table.Row("web", "line1\nline2\tend")
	if err := table.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	body := buf.String()
	if bytes.Count([]byte(body), []byte("\n")) != 2 {
		t.Fatalf("a multi-line cell must stay on one row, got:\n%s", body)
	}
	if !bytes.Contains([]byte(body), []byte(`line1\nline2\tend`)) {
		t.Fatalf("control characters must be escaped, got:\n%s", body)
	}
}
