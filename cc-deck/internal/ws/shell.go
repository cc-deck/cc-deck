package ws

import "strings"

// shellJoin quotes each argument and joins them with spaces, producing
// a string safe to pass as a remote shell command.
func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = shellQuote(a)
	}
	return strings.Join(quoted, " ")
}
