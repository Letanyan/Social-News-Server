package main

import "time"

func utc() time.Time {
	return time.Now().UTC()
}

func idFromTime(t time.Time) int64 {
	y := int64(t.Year()) % 1000
	m := int64(t.Month())             // 2
	d := int64(t.Day())               // 2
	h := int64(t.Hour())              // 2
	n := int64(t.Minute())            // 2
	s := int64(t.Second())            // 2
	z := int64(t.Nanosecond()) / 1000 // 6
	info.Println(y)
	info.Println(y * 1_00_00_00_00_00_000000)
	info.Println(m * 1_00_00_00_00_000000)
	info.Println(d * 1_00_00_00_000000)
	info.Println(h * 1_00_00_000000)
	info.Println(n * 1_00_000000)
	info.Println(s * 1_000000)
	info.Println(z * 1)
	return y*1_00_00_00_00_00_000000 + m*1_00_00_00_00_000000 + d*1_00_00_00_000000 +
		h*1_00_00_000000 + n*1_00_000000 + s*1_000000 + z
}

func yearFromId(id int64) int64 {
	info.Println(id)
	info.Println(id / 1_00_00_00_00_00_000000)
	return id/1_00_00_00_00_00_000000 + 2000
}

func yearsBetweenDates(sDate time.Time, eDate time.Time) []int64 {
	result := []int64{}
	for sDate.Year() <= eDate.Year() {
		result = append(result, int64(sDate.Year()))
		sDate = sDate.AddDate(1, 0, 0)
	}
	return result
}
