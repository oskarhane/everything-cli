package podcast

import (
	"bytes"
	"strconv"
	"strings"
)

// parseCues parses a transcript body that is either WebVTT (WEBVTT magic) or
// SubRip (SRT) into Segments. The format is chosen by content sniffing — the
// presence of the WEBVTT magic bytes — never by a declared MIME type. VTT cues
// carry MM:SS.mmm dot-millis timestamps (with optional leading hours) while
// SRT cues carry HH:MM:SS,mmm comma-millis timestamps; the shared line engine
// accepts both. A body with no parseable cues yields an empty (non-nil) slice.
func parseCues(data []byte) ([]Segment, error) {
	body := bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // allow a UTF-8 BOM
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, nil
	}
	lines := splitTranscriptLines(trimmed)
	if bytes.HasPrefix(trimmed, []byte("WEBVTT")) {
		return parseVTT(lines), nil
	}
	return parseSRT(lines), nil
}

// parseVTT parses a WebVTT body. Cues and payloads follow the same shape as
// SRT, so both delegate to cueLines.
func parseVTT(lines []string) []Segment { return cueLines(lines) }

// parseSRT parses a SubRip body.
func parseSRT(lines []string) []Segment { return cueLines(lines) }

// cueLines parses timestamp blocks from already-split transcript lines. A
// block starts with a timestamp timing line, followed by one or more payload
// lines up to a blank line. Header blocks, cue identifiers, and NOTE text are
// ignored (they are never timing lines and appear outside an open cue).
// Cues with an empty payload are dropped.
func cueLines(lines []string) []Segment {
	var segs []Segment
	var start, end int64
	var have bool
	var cur []string
	for _, line := range lines {
		if s, e, ok := parseCueLine(line); ok {
			if have {
				segs = appendCue(segs, start, end, cur)
			}
			start, end, have = s, e, true
			cur = nil
			continue
		}
		if strings.TrimSpace(line) == "" {
			if have {
				segs = appendCue(segs, start, end, cur)
				have = false
				cur = nil
			}
			continue
		}
		if have {
			cur = append(cur, line)
		}
	}
	if have {
		segs = appendCue(segs, start, end, cur)
	}
	return segs
}

// appendCue builds a Segment from a cue's timestamps and (possibly multiline)
// payload, skipping cues whose payload is empty or whitespace-only.
func appendCue(segs []Segment, start, end int64, text []string) []Segment {
	t := strings.TrimSpace(strings.Join(text, "\n"))
	if t == "" {
		return segs
	}
	duration := end - start
	if duration < 0 {
		duration = 0
	}
	return append(segs, Segment{StartMS: start, DurationMS: duration, Text: t})
}

// parseCueLine parses a cue timing line of the form
//
//	start --> end
//
// where start/end are VTT (MM:SS.mmm or H:MM:SS.mmm) or SRT (HH:MM:SS,mmm)
// timestamps. Trailing cue settings after the end timestamp are ignored.
func parseCueLine(line string) (start, end int64, ok bool) {
	idx := strings.Index(line, "-->")
	if idx < 0 {
		return 0, 0, false
	}
	startTok := strings.TrimSpace(line[:idx])
	endParts := strings.Fields(line[idx+3:])
	if len(endParts) == 0 {
		return 0, 0, false
	}
	s, ok1 := parseTimestamp(startTok)
	e, ok2 := parseTimestamp(endParts[0])
	if !ok1 || !ok2 {
		return 0, 0, false
	}
	return s, e, true
}

// parseTimestamp converts a single VTT/SRT timestamp to milliseconds. Both the
// dot-millis (VTT "SS.mmm") and comma-millis (SRT "SS,mmm") forms are
// accepted, as are 1-3 colon-separated fields (SS, MM:SS, or H:MM:SS).
func parseTimestamp(tok string) (int64, bool) {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return 0, false
	}
	parts := strings.Split(tok, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	var ms int64
	for _, p := range parts[:len(parts)-1] {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return 0, false
		}
		ms = ms*60 + int64(v)*60000
	}
	// Last field is seconds with an optional fractional .mmm / ,mmm suffix.
	secStr := strings.TrimSpace(parts[len(parts)-1])
	secStr = strings.Replace(secStr, ",", ".", 1)
	sec := secStr
	fracStr := ""
	if dot := strings.Index(sec, "."); dot >= 0 {
		fracStr = sec[dot+1:]
		sec = sec[:dot]
	}
	secVal, err := strconv.Atoi(sec)
	if err != nil {
		return 0, false
	}
	ms += int64(secVal) * 1000
	if fracStr != "" {
		if len(fracStr) > 3 {
			fracStr = fracStr[:3]
		}
		for len(fracStr) < 3 {
			fracStr += "0"
		}
		f, err := strconv.Atoi(fracStr)
		if err != nil {
			return 0, false
		}
		ms += int64(f)
	}
	return ms, true
}

// splitTranscriptLines splits a transcript body into lines, normalizing CRLF
// endings.
func splitTranscriptLines(data []byte) []string {
	lines := strings.Split(string(data), "\n")
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	return lines
}
