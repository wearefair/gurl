package util

import "strings"

// Trims starting "." because message names from the input types come in with a "." prepended
func NormalizeMessageName(name string) string {
	return strings.TrimLeft(name, ".")
}
