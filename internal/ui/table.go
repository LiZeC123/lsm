// Package ui renders the plain text tables and interactive prompts used by the
// lsm CLI.
package ui

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"unicode"
)

// Table renders fixed width, space aligned columns (dev.md §5.5, §5.6).
type Table struct {
	tw *tabwriter.Writer
}

// NewTable starts a table on w and immediately writes the header row.
func NewTable(w io.Writer, headers ...string) *Table {
	t := &Table{tw: tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)}
	t.Row(headers...)
	return t
}

// Row appends one data row.
func (t *Table) Row(cells ...string) {
	escaped := make([]string, len(cells))
	for i, cell := range cells {
		escaped[i] = escapeCell(cell)
	}
	fmt.Fprintln(t.tw, strings.Join(escaped, "\t"))
}

// Flush renders every buffered row.
func (t *Table) Flush() error { return t.tw.Flush() }

// escapeCell replaces control characters with visible escapes. A command taken
// from the config may legitimately contain tabs or newlines (YAML block scalars)
// and those would otherwise destroy the table layout.
func escapeCell(cell string) string {
	if !strings.ContainsFunc(cell, unicode.IsControl) {
		return cell
	}
	var b strings.Builder
	b.Grow(len(cell))
	for _, r := range cell {
		switch {
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case unicode.IsControl(r):
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
