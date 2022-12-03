package main

import (
	"database/sql"
	"fmt"
)

type Location struct {
	Id   int64  `json:"id"`
	Name string `json:"name"`
}

func DBCreateLocation(db *sql.DB, location []string) []int64 {
	if len(location) <= 0 {
		return []int64{}
	}

	dbLocations := DBGetLocation(db, location)
	newLocations := []string{}
	for _, loc := range location {
		contains := false
		for _, dbLoc := range dbLocations {
			if dbLoc.Name == loc {
				contains = true
				break
			}
		}
		if !contains {
			newLocations = append(newLocations, loc)
		}
	}

	locationRows := SQLFormattedRows(newLocations, func(s string) string {
		return ""
	})
	locationArray := SQLFormattedArray(newLocations)

	result := []int64{}
	if len(newLocations) > 0 {
		query := fmt.Sprintf(`
		INSERT INTO Location(name)
		VALUES %s ON CONFLICT (name) DO NOTHING;
		SELECT id
		FROM Location
		WHERE Array[name] <@ %s;
		`, locationRows, locationArray)

		rows, e := db.Query(query)
		if DidFail(e, "create tags", query) {
			return []int64{}
		}
		defer rows.Close()
		var id int64
		for rows.Next() {
			rows.Scan(&id)
			result = append(result, id)
		}
	}

	finalResult := []int64{}
	for _, loc := range dbLocations {
		finalResult = append(finalResult, loc.Id)
	}
	finalResult = append(finalResult, result...)

	return finalResult
}

func DBGetLocation(db *sql.DB, location []string) []Location {
	locationArray := SQLFormattedArray(location)
	query := fmt.Sprintf(`
	SELECT id, name
	FROM Location
	WHERE Array[name] <@ %s;
	`, locationArray)
	rows, e := db.Query(query)
	if DidFail(e, "create tags") {
		return []Location{}
	}
	defer rows.Close()
	result := []Location{}
	var loc Location
	for rows.Next() {
		rows.Scan(&loc.Id, &loc.Name)
		result = append(result, loc)
	}

	return result
}

func DBGetLocationIndex(db *sql.DB, location []string) string {
	indices := DBCreateLocation(db, location)
	return SQLFormattedIndexArray(indices)
}
