package main

import "time"

func utc() time.Time {
	return time.Now().UTC()
}

func yearsBetweenDates(sDate time.Time, eDate time.Time) []int64 {
	result := []int64{}
	for sDate.Year() <= eDate.Year() {
		result = append(result, int64(sDate.Year()))
		sDate = sDate.AddDate(1, 0, 0)
	}
	return result
}
