package render

import "strings"

// fieldPrefixes are the line prefixes whose labels we color.
// We only style the prefix; the content after the colon is printed verbatim.
var fieldPrefixes = []string{
	"Login:",
	"Login name:",
	"Name:",
	"In real life:",
	"Plan:",
	"Project:",
	"Office:",
	"Office Phone:",
	"Home Phone:",
	"Directory:",
	"Shell:",
	"Last login",
	"Mail last read",
	"New mail received",
	"No mail.",
	"Never logged in.",
	"No Plan.",
	"On since",
}

// highlightFields walks body line by line and re-emits each line. If a line
// begins with one of fieldPrefixes (or extra), the prefix is wrapped in
// theme.Field; the rest of the line is untouched.
func highlightFields(theme Theme, body []byte, extra []string) string {
	if theme.NoColor {
		return string(body)
	}
	lines := strings.SplitAfter(string(body), "\n")
	var sb strings.Builder
	for _, line := range lines {
		matched := false
		for _, prefix := range fieldPrefixes {
			if strings.HasPrefix(line, prefix) {
				sb.WriteString(theme.Field.Render(prefix))
				sb.WriteString(line[len(prefix):])
				matched = true
				break
			}
		}
		if !matched {
			for _, prefix := range extra {
				if strings.HasPrefix(line, prefix) {
					sb.WriteString(theme.Field.Render(prefix))
					sb.WriteString(line[len(prefix):])
					matched = true
					break
				}
			}
		}
		if !matched {
			sb.WriteString(line)
		}
	}
	return sb.String()
}
