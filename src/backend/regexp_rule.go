package magitrickle

import (
	"regexp"
)

type RegexpRule struct {
	Re    *regexp.Regexp
	Group *Group
}
