package doc

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// splitFrontMatter splits the source into (front-matter YAML body, remaining
// markdown). If no front-matter block is present, returns ("", src).
//
// Front matter is a "---" line, then YAML, then a closing "---" line. The
// opening fence must be the very first line of the file.
func splitFrontMatter(src string) (string, string) {
	if !strings.HasPrefix(src, "---\n") && !strings.HasPrefix(src, "---\r\n") {
		return "", src
	}
	rest := src[len("---"):]
	// Skip the line ending after the opener.
	if strings.HasPrefix(rest, "\r\n") {
		rest = rest[2:]
	} else if strings.HasPrefix(rest, "\n") {
		rest = rest[1:]
	}
	// Find the closing fence (a line that is exactly "---").
	end := -1
	for i := 0; i < len(rest); {
		// Scan to end of current line.
		j := strings.IndexByte(rest[i:], '\n')
		var line string
		var lineEnd int
		if j < 0 {
			line = rest[i:]
			lineEnd = len(rest)
		} else {
			line = rest[i : i+j]
			lineEnd = i + j + 1
		}
		line = strings.TrimRight(line, "\r")
		if line == "---" {
			end = i
			rest = rest[lineEnd:]
			break
		}
		if j < 0 {
			break
		}
		i = lineEnd
	}
	if end < 0 {
		return "", src // unterminated — treat whole thing as body
	}
	body := rest
	return strings.TrimRight(strings.TrimRight(srcSlice(src, 4, 4+end), "\r"), "\n"), body
}

// srcSlice returns src[from:to] safely.
func srcSlice(src string, from, to int) string {
	if from > len(src) {
		from = len(src)
	}
	if to > len(src) {
		to = len(src)
	}
	if to < from {
		to = from
	}
	return src[from:to]
}

// parseMeta unmarshals the YAML body into a DocMeta. Unknown keys are
// ignored.
func parseMeta(body string) DocMeta {
	if strings.TrimSpace(body) == "" {
		return DocMeta{}
	}
	raw := struct {
		Instructions string `yaml:"instructions"`
	}{}
	_ = yaml.Unmarshal([]byte(body), &raw)
	return DocMeta{Instructions: raw.Instructions}
}

// writeMeta serializes meta as YAML, returning "" when meta is empty so we
// don't emit a useless empty front-matter block.
func writeMeta(m DocMeta) string {
	if strings.TrimSpace(m.Instructions) == "" {
		return ""
	}
	raw := struct {
		Instructions string `yaml:"instructions,omitempty"`
	}{Instructions: m.Instructions}
	out, err := yaml.Marshal(raw)
	if err != nil {
		return ""
	}
	return string(out)
}
