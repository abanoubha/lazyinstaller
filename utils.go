package main

import (
	"fmt"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// convert special characters into the standard ones
// such as convert ã into a
func normalizeText(input string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, err := transform.String(t, input)
	if err != nil {
		fmt.Println("error normalizing this text (", input, "), err:", err)
		return ""
	}
	return result
}
