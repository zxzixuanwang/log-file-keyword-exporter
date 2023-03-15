package filter

import (
	"strings"
)

type DefaultFilter struct {
}

func NewFilter(df DefaultFilter) HaveFilterInterface[string] {
	return &df
}

func (df *DefaultFilter) HaveFilter(msg string, keyWord []string) *string {
	check := false
	for _, v := range keyWord {
		v := v
		check = strings.Contains(msg, v)
		if check {

			return &v
		}
	}
	return nil
}
