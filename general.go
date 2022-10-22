package main

import (
	"regexp"
	"unicode"
)

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

func validMatch(value string, pattern string) bool {
	regex, e := regexp.Compile(pattern)
	if DidFail(e, "build regex pattern", pattern) {
		return false
	}
	return regex.Match([]byte(value))
}

func validEmailMatch(value string) bool {
	return validMatch(value, `\b[\w.!#$%&’*+\/=?^`+"`"+`{|}~-]+@[\w-]+(?:\.[\w-]+)*\b`)
}
