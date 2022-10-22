package main

import (
	"fmt"
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

func isNotAlphanumericWhitespace(c rune) bool {
	return !(unicode.IsLetter(c) || unicode.IsNumber(c) || unicode.IsSpace(c))
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

func DBPrepareSearchString(query string) (string, []int64) {
	tagRe, e := regexp.Compile(`#\(?([\w\d\s]+)\)?`)
	tagNames := []string{}
	tags := []int64{}
	if DidFail(e, "compile tag regex") {
		query = replaceUnicode(query, isNotAlphanumericWhitespace)
		return query, tags
	}
	query = tagRe.ReplaceAllStringFunc(query, func(m string) string {
		fmt.Println(m)
		if m[1] == '(' {
			m = m[2:]
		} else {
			m = m[1:]
		}
		if m[len(m)-1] == ')' {
			m = m[:len(m)-1]
		}
		tagNames = append(tagNames, m)
		return ""
	})
	query = replaceUnicode(query, isNotAlphanumericWhitespace)

	tagObjs := DBGetTags(mainDB, -1, tagNames, []string{}, 0, 0, soUpvotes, int64(len(tagNames)), 0, "", "", 0, "")
	for _, t := range tagObjs {
		tags = append(tags, t.ID)
	}

	return query, tags
}
