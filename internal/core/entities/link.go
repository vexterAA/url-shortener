package entities

import "strings"

const (
	CodeLength   = 10
	CodeAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_"
)

type Link struct {
	Code string
	URL  string
}

func IsCodeValid(code string) bool {
	if len(code) != CodeLength {
		return false
	}

	for i := range len(code) {
		if strings.IndexByte(CodeAlphabet, code[i]) == -1 {
			return false
		}
	}

	return true
}
