package base

import (
	"strings"
)

// PathSplitQuery splits a path from a query.
func PathSplitQuery(pathAndQuery string) (string, string) {
	i := strings.Index(pathAndQuery, "?")
	if i >= 0 {
		return pathAndQuery[:i], pathAndQuery[i+1:]
	}

	return pathAndQuery, ""
}
