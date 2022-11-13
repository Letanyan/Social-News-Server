package main

import (
	"encoding/gob"
	"os"
	"regexp"
	"strings"
	"sync"
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

func tagFormat(original string) []string {
	punct := func(r rune) bool { return unicode.IsPunct(r) && r != ',' }
	lower := func(r rune) rune { return unicode.ToLower(r) }
	formatted := filterMapUnicode(original, punct, lower)
	splits := strings.Split(formatted, ",")
	result := []string{}
	for _, s := range splits {
		s = strings.TrimSpace(s)
		if len(s) > 0 {
			result = append(result, s)
		}
	}
	return result
}

func isNotWebSearchQuery(c rune) bool {
	return !(unicode.IsLetter(c) || unicode.IsNumber(c) || unicode.IsSpace(c) || c == '-' || c == '"')
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

func DBPrepareTaggedString(text string, removeOnlyHash bool) (string, []string) {
	tagRe, e := regexp.Compile(`#\(?([\w\d\s]+)\)?`)
	tagNames := []string{}
	tags := []string{}
	if DidFail(e, "compile tag regex") {
		text = replaceUnicode(text, isNotWebSearchQuery)
		return text, tags
	}
	text = tagRe.ReplaceAllStringFunc(text, func(m string) string {
		if m[1] == '(' {
			m = m[2:]
		} else {
			m = m[1:]
		}
		if m[len(m)-1] == ')' {
			m = m[:len(m)-1]
		}
		tagNames = append(tagNames, m)
		if removeOnlyHash {
			return m
		} else {
			return ""
		}
	})

	return text, tagNames
}

func DBPrepareSearchString(query string) (string, []int64) {
	text, tagNames := DBPrepareTaggedString(query, false)
	text = replaceUnicode(text, isNotWebSearchQuery)
	tagObjects := DBGetTags(mainDB, -1, tagNames, []string{}, 0, 0, soUpvotes, int64(len(tagNames)), 0, "", "", 0, "")
	tags := []int64{}
	for _, t := range tagObjects {
		tags = append(tags, t.ID)
	}

	return text, tags
}

func AUTHLoadFromFile(fileName string) map[int64][]string {
	result := map[int64][]string{}
	file, e := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR, 0644)
	if DidFail(e, "open file ", fileName) {
		return result
	}
	defer file.Close()

	dec := gob.NewDecoder(file)
	e = dec.Decode(&result)
	if DidFail(e, "decode file ", fileName, " to hash map") {
		return result
	}
	return result
}

func AUTHWriteToFile(data map[int64][]string, fileName string) {
	file, e := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 0644)
	if DidFail(e, "open file ", fileName) {
		return
	}
	defer file.Close()
	enc := gob.NewEncoder(file)
	e = enc.Encode(data)
	if DidFail(e, "gob write file") {
		return
	}
}

func Zero[T any]() T {
	var result T
	return result
}

type KeyedMutex struct {
	mutexes sync.Map // Zero value is empty and ready for use
}

func (m *KeyedMutex) Lock(key string) func() {
	value, _ := m.mutexes.LoadOrStore(key, &sync.Mutex{})
	mtx := value.(*sync.Mutex)
	mtx.Lock()

	return func() { mtx.Unlock() }
}
