package main

import "time"

func utc() time.Time {
	return time.Now().UTC()
}
