package main

import (
	"fmt"
	"strings"
	"time"
)

func formatTime(t time.Time) string {
	nowTime := t.Format("2006-01-02 15:04:05.999999")
	return nowTime
}

func parseTime(t string) time.Time {
	result, e := time.Parse("2006-01-02 15:04:05.999999", t)
	DidFail(e, "parsing time", t)
	return result
}

func formatNow() string {
	t := time.Now().UTC()
	return formatTime(t)
}

func sign(t bool) int64 {
	if t {
		return 1
	} else {
		return -1
	}
}

func SQLFormattedArray(array []string) string {
	result := "'{"
	for i, s := range array {
		t := strings.ReplaceAll(s, "'", "")
		t = strings.ReplaceAll(t, "\"", "")
		result += fmt.Sprintf("\"%s\"", t)
		if i < len(array)-1 {
			result += ","
		}
	}
	result += "}'"
	return result
}

func SQLFormattedIndexArray(array []int64) string {
	result := "'{"
	for i, s := range array {
		result += fmt.Sprintf("\"%d\"", s)
		if i < len(array)-1 {
			result += ","
		}
	}
	result += "}'"
	return result
}

func SQLFormattedRows(array []string, rest func(string) string) string {
	result := ""
	for i, s := range array {
		t := strings.ReplaceAll(s, "'", "")
		t = strings.ReplaceAll(t, "\"", "")
		r := rest(t)
		if len(r) > 0 {
			result += fmt.Sprintf("('%s',"+r+")", t)
		} else {
			result += fmt.Sprintf("('%s')", t)
		}
		if i < len(array)-1 {
			result += ","
		}
	}
	return result
}

func SQLFormattedIndexRows(array []int64, rest func(int64) string) string {
	result := ""
	for i, s := range array {
		r := rest(s)
		if len(r) > 0 {
			result += fmt.Sprintf("(%d,"+r+")", s)
		} else {
			result += fmt.Sprintf("(%d)", s)
		}
		if i < len(array)-1 {
			result += ","
		}
	}
	return result
}
