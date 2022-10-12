package main

import "unicode"

func replaceUnicode(original string, shouldRemove func(rune) bool) string {
	result := ""
	for _, c := range original {
		if !shouldRemove(c) {
			result += string(c)
		}
	}
	return result
}

func filterMapUnicode(original string, shouldRemove func(rune) bool, mapping func(rune) rune) string {
	result := ""
	for _, c := range original {
		if !shouldRemove(c) {
			result += string(mapping(c))
		}
	}
	return result
}

func tagFormat(original string) string {
	punct := func(r rune) bool { return unicode.IsPunct(r) }
	lower := func(r rune) rune { return unicode.ToLower(r) }
	return filterMapUnicode(original, punct, lower)
}

func isNotAlphanumeric(c rune) bool {
	return !(unicode.IsLetter(c) || unicode.IsNumber(c))
}

func ContainsItem[I comparable](item I, list []I) bool {
	for _, x := range list {
		if x == item {
			return true
		}
	}
	return false
}
