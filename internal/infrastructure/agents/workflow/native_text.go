package workflow

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// nativeText renders native JSON strings as supplied text so exact citations do
// not depend on JSON newline, quote, or HTML escaping. The structured record is
// supplied separately; labels here identify native fields without judging them.
func nativeText(content []byte) string {
	var value any
	if err := json.Unmarshal(content, &value); err != nil {
		return string(content)
	}
	var text strings.Builder
	writeNativeText(&text, value)
	return text.String()
}

func writeNativeText(text *strings.Builder, value any) {
	switch typed := value.(type) {
	case string:
		text.WriteString(typed)
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			text.WriteString(key + ":\n")
			writeNativeText(text, typed[key])
			text.WriteByte('\n')
		}
	case []any:
		for index, member := range typed {
			fmt.Fprintf(text, "[%d]:\n", index)
			writeNativeText(text, member)
			text.WriteByte('\n')
		}
	default:
		fmt.Fprint(text, value)
	}
}
