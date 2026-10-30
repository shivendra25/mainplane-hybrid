package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/mainplane-ai/mainplane/pkg/statefile"
)

// maxIndex caps the body of the newest index record. Only the newest index
// record stays whole in the context, so the cap keeps it a small fraction of
// a step's prompt; every entry cut from it is still in the log file, and a
// narrower record range reaches the same bytes.
const maxIndex = 4 << 10

// The logs tool reports at most maxMatches matching records, maxLines lines
// of each, and at most matchB bytes around each match. A bounded report
// teaches the model to narrow a range and search again; the log holds it all.
const (
	maxMatches = 8
	maxLines   = 12
	matchB     = 240
)

// indexRecord is the active state index, one line per call of a step: the
// record range that holds the raw bytes, what the call named, the size of the
// result, its exit. Cheap to build, needs no model, and exact: the map of the
// session the model keeps in context to know what to search for.
func indexRecord(lines []string) statefile.Record {
	for i := range lines { // keep the newest entries that fit; the log keeps the rest
		if len(strings.Join(lines[i:], "\n")) <= maxIndex {
			if i > 0 {
				lines = append([]string{fmt.Sprintf("[first %d entries cut, search the log]", i)}, lines[i:]...)
			} else {
				lines = lines[i:]
			}
			break
		}
	}
	return statefile.Record{Header: statefile.Header{Kind: statefile.Index, Type: "text/plain"}, Body: []byte(strings.Join(lines, "\n"))}
}

// lineOf is one index line: the call and its result as the model reads it.
func lineOf(c statefile.Record, res statefile.Record, rn int) string {
	var d struct {
		Name      string
		Arguments args
	}
	if err := json.Unmarshal(c.Body, &d); err != nil {
		d.Name = "call"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "recs %d-%d %s", c.N, rn, d.Name)
	switch d.Name {
	case "run":
		if d.Arguments.Worker != "" {
			fmt.Fprintf(&b, " on %s %q", d.Arguments.Worker, digest([]byte(d.Arguments.Code)))
		}
	case "read", "write", "edit":
		if d.Arguments.Path != "" {
			b.WriteString(" " + d.Arguments.Path)
		}
	case "patch":
		b.WriteString(" " + digest([]byte(d.Arguments.Input)))
	}
	fmt.Fprintf(&b, " %dB", len(res.Body))
	if res.Exit != nil && *res.Exit != 0 {
		fmt.Fprintf(&b, " exit %d", *res.Exit)
	}
	return b.String()
}

// digest is the first line of a call's code or input, so the index line says
// what the call did without quoting all of it.
func digest(b []byte) string {
	line, _, _ := bytes.Cut(bytes.TrimSpace(b), []byte("\n"))
	line = bytes.TrimSpace(line)
	if len(line) > 48 {
		line = line[:48]
		for !utf8.Valid(line) {
			line = line[:len(line)-1]
		}
		line = append(line, "..."...)
	}
	return string(line)
}

// searchLogs is the logs tool: the raw record bodies of a session's log file
// pulled back into the context on demand. Retrieval is exact, never a
// summary, and the record range comes from the index.
func (h *Harness) searchLogs(id string, a args) (statefile.Record, int) {
	target := id
	if a.Session != "" {
		target = a.Session
		if !statefile.ValidID(target) || target == id {
			return result("", "error: session must be the 8 hex digit id of another session"), 0
		}
	}
	chain, err := h.Sessions.Load(target)
	if err != nil {
		return result("", "error: "+err.Error()), 0
	}
	if a.To != 0 && a.To < a.From {
		return result("", fmt.Sprintf("error: to %d is before from %d", a.To, a.From)), 0
	}
	q := []byte(strings.ToLower(a.Query))
	var b strings.Builder
	shown, hits := 0, 0
	for _, r := range chain {
		if r.N < a.From || (a.To != 0 && r.N > a.To) {
			continue
		}
		switch r.Kind {
		case statefile.Start, statefile.Config, statefile.Name:
			continue // start, config, name hold no content to search
		}
		if !bytes.Contains(bytes.ToLower(r.Body), q) {
			continue
		}
		hits++
		if shown == maxMatches {
			continue
		}
		shown++
		show(&b, r, q)
	}
	if hits > maxMatches {
		fmt.Fprintf(&b, "[%d more matching records, set from and to to narrow]\n", hits-maxMatches)
	}
	if shown == 0 {
		return result("", fmt.Sprintf("no match for %q in records %d..%d", a.Query, max(1, a.From), capTo(chain, a.To))), 0
	}
	return statefile.Record{Header: statefile.Header{Type: "text/plain"}, Body: []byte(b.String())}, 0
}

// capTo is the top of an open record range, for a message that names it.
func capTo(chain []statefile.Record, to int) int {
	if to == 0 {
		return chain[len(chain)-1].N
	}
	return to
}

// show prints a record as the model reads it: one header line, then the
// lines that hold the match, each cut to the match's window in a long line.
func show(b *strings.Builder, r statefile.Record, q []byte) {
	fmt.Fprintf(b, "rec %d %s", r.N, r.Kind)
	if r.For != "" {
		fmt.Fprintf(b, " for=%s", r.For)
	}
	if r.Exit != nil {
		fmt.Fprintf(b, " exit=%d", *r.Exit)
	}
	if r.Truncated {
		b.WriteString(" truncated")
	}
	fmt.Fprintf(b, " %dB\n", len(r.Body))
	n := 0
	for _, ln := range bytes.Split(bytes.TrimSuffix(r.Body, []byte("\n")), []byte("\n")) {
		if n >= maxLines {
			break
		}
		i := bytes.Index(bytes.ToLower(ln), q)
		if i < 0 {
			continue
		}
		if n > 0 {
			b.WriteString("...\n")
		}
		lo, hi := window(ln, i, matchB)
		b.Write(ln[lo:hi])
		b.WriteByte('\n')
		n++
	}
	if n == 0 { // the match spans lines; show its window raw
		b.WriteString(sample(r.Body, q) + "\n")
	}
	b.WriteByte('\n')
}

// window is the bytes around a match in one line, grown from both ends.
func window(ln []byte, i, budget int) (int, int) {
	lo, hi := i, i
	for hi-lo < budget && (lo > 0 || hi < len(ln)) {
		if lo > 0 {
			lo--
		}
		if hi < len(ln) && hi-lo < budget {
			hi++
		}
	}
	return lo, hi
}

// sample is a window printed as one line, so a match across lines is still
// readable inside the report.
func sample(b []byte, q []byte) string {
	i := bytes.Index(bytes.ToLower(b), q)
	if i < 0 {
		i = 0
	}
	lo, hi := window(b, i, matchB)
	return string(bytes.ReplaceAll(b[lo:hi], []byte("\n"), []byte(" ")))
}
