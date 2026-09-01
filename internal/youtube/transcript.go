package youtube

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"math"
	"strconv"
	"strings"
)

// parseTranscriptBody turns a timedtext response body into Segments. It
// accepts the modern XML shape (<p t="ms" d="ms">), the legacy XML shape
// (<text start="sec" dur="sec">), and fmt=json3 JSON payloads. A body with
// no parseable content yields a nil slice (the caller maps it to
// ErrEmptyTranscript).
func parseTranscriptBody(data []byte) ([]Segment, error) {
	body := bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // allow a UTF-8 BOM
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, nil
	}
	switch trimmed[0] {
	case '<':
		return parseTranscriptXML(trimmed)
	case '{':
		return parseTranscriptJSON3(trimmed)
	default:
		return nil, fmt.Errorf("unrecognized transcript format (first byte %q)", trimmed[0])
	}
}

// parseTranscriptXML parses both timedtext XML shapes into identical
// Segments: <p t="..." d="..."> carries milliseconds directly, while the
// older <text start="..." dur="..."> carries seconds that are rounded to
// milliseconds. Segment text is HTML-unescaped; multiline text within a
// segment is concatenated.
func parseTranscriptXML(data []byte) ([]Segment, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var segs []Segment
	var cur *Segment
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				cur = &Segment{StartMS: msAttr(t, "t"), DurationMS: msAttr(t, "d")}
			case "text":
				cur = &Segment{StartMS: secAttrToMS(t, "start"), DurationMS: secAttrToMS(t, "dur")}
			}
		case xml.CharData:
			if cur != nil {
				cur.Text += string(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "p", "text":
				if cur == nil {
					continue
				}
				cur.Text = strings.TrimSpace(html.UnescapeString(cur.Text))
				segs = append(segs, *cur)
				cur = nil
			}
		}
	}
	return segs, nil
}

// parseTranscriptJSON3 parses a fmt=json3 timedtext payload
// ({"events":[{"tStartMs":...,"dDurationMs":...,"segs":[{"utf8":...}]}]}).
func parseTranscriptJSON3(data []byte) ([]Segment, error) {
	var doc json3Transcript
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	var segs []Segment
	for _, ev := range doc.Events {
		var text strings.Builder
		for _, s := range ev.Segs {
			text.WriteString(s.UTF8)
		}
		if text.Len() == 0 {
			continue
		}
		segs = append(segs, Segment{
			StartMS:    ev.TStartMS,
			DurationMS: ev.DDurationMS,
			Text:       html.UnescapeString(text.String()),
		})
	}
	return segs, nil
}

type json3Transcript struct {
	Events []struct {
		TStartMS    int64 `json:"tStartMs"`
		DDurationMS int64 `json:"dDurationMs"`
		Segs        []struct {
			UTF8 string `json:"utf8"`
		} `json:"segs"`
	} `json:"events"`
}

// msAttr reads an integer attribute (milliseconds) from an XML element,
// defaulting to 0 when absent or malformed.
func msAttr(se xml.StartElement, name string) int64 {
	for _, a := range se.Attr {
		if a.Name.Local == name {
			v, err := strconv.ParseInt(a.Value, 10, 64)
			if err == nil {
				return v
			}
		}
	}
	return 0
}

// secAttrToMS reads a float attribute (seconds) from an XML element and
// rounds it to milliseconds, defaulting to 0 when absent or malformed.
func secAttrToMS(se xml.StartElement, name string) int64 {
	for _, a := range se.Attr {
		if a.Name.Local == name {
			f, err := strconv.ParseFloat(a.Value, 64)
			if err != nil {
				return 0
			}
			return int64(math.Round(f * 1000))
		}
	}
	return 0
}
