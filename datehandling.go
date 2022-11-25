package main

import (
	"time"
)

func utc() time.Time {
	return time.Now().UTC()
}

func sanitizeDate(text string) string {
	for _, c := range text {
		switch c {
		case ' ', '.', '-', '+', ':', 'T', 'Z', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':

		default:
			return ""
		}
	}
	return text
}
