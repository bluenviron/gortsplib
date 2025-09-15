package url

import (
	"github.com/bluenviron/gortsplib/v5/pkg/base"
)

// PathSplitQuery splits a path from a query.
//
// Deprecated: replaced by base.PathSplitQuery
func PathSplitQuery(pathAndQuery string) (string, string) {
	return base.PathSplitQuery(pathAndQuery)
}
