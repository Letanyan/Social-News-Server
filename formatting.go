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

func formatTimestamp(t time.Time) string {
	nowTime := fmt.Sprintf("%d", t.UnixNano())
	return nowTime
}

func parseUnknownTime(t string) time.Time {
	result, e := time.Parse("2006-01-02T15:04:05.999999Z", t)
	if e == nil {
		return result
	}
	result, e = time.Parse("2006-01-02T15:04:05.999999+0000", t)
	if e == nil {
		return result
	}
	result, e = time.Parse("2006-01-02T15:04:05.999999", t)
	if e == nil {
		return result
	}
	result, e = time.Parse("2006-01-02 15:04:05.999999", t)
	if e == nil {
		return result
	}
	result, e = time.Parse("2006-01-02T15:04:05Z", t)
	if e == nil {
		return result
	}
	result, e = time.Parse("2006-01-02T15:04:05+0000", t)
	if e == nil {
		return result
	}
	result, e = time.Parse("2006-01-02T15:04:05", t)
	if e == nil {
		return result
	}
	result, e = time.Parse("2006-01-02 15:04:05", t)
	if e == nil {
		return result
	}
	return utc()
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
		result += fmt.Sprintf("%d", s)
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

func SQLFormattedIndexList(array []int64, rest func(int64) string) string {
	result := ""
	for i, s := range array {
		result += rest(s)
		if i < len(array)-1 {
			result += ","
		}
	}
	return result
}

func ReplaceDateValues(query string, date string) string {
	if len(date) > 0 {
		query = strings.ReplaceAll(query, "{date}", ", updatedAt")
		query = strings.ReplaceAll(query, "{date_value}", ", '"+date+"'")
		query = strings.ReplaceAll(query, "{date_value_res}", "'"+date+"'")
	} else {
		query = strings.ReplaceAll(query, "{date}", "")
		query = strings.ReplaceAll(query, "{date_value}", "")
		query = strings.ReplaceAll(query, "{date_value_res}", "'"+utc().Format("2006-01-02")+"'")
	}
	return query
}

func JoinStrings(list []string, joiner string) string {
	result := ""
	for i, s := range list {
		result += s
		if i < len(list)-1 {
			result += joiner
		}
	}
	return result
}
