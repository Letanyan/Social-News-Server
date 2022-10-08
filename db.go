package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func DBSetup(db *sql.DB) {
	createUsers := `CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL,
		name VARCHAR(21) NOT NULL,
		email TEXT NOT NULL,
		password TEXT NOT NULL,
		registerDate TIMESTAMP DEFAULT now(),
		updatedAt TIMESTAMP DEFAULT now(),
		upvotes DOUBLE PRECISION DEFAULT 0.0,
		downvotes DOUBLE PRECISION DEFAULT 0.0,
		credits INTEGER DEFAULT 25,
		validationKey BIGINT NOT NULL,

		PRIMARY KEY (id)
	);`
	_, e := db.Exec(createUsers)
	DidFail(e, "create users table")

	year := time.Now().UTC().Year()
	createPostsTable := func(year int) {
		createPosts := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS posts%d (
			id BIGSERIAL NOT NULL,
			userId BIGINT NOT NULL,
			content TEXT,
			tags TEXT[],
			createdAt TIMESTAMP DEFAULT now(),
			updatedAt TIMESTAMP DEFAULT now(),
			upvotes DOUBLE PRECISION DEFAULT 0.0,
			downvotes DOUBLE PRECISION DEFAULT 0.0,
			location TEXT[],
	
			PRIMARY KEY (id)
		);`, year)
		_, e = db.Exec(createPosts)
		DidFail(e, "create posts table")
	}
	createPostsTable(year)
	createPostsTable(year + 1)

	createCommentsTable := func(year int) {
		createCommentsTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS Comments%d(
			id BIGSERIAL NOT NULL,
			postId BIGINT,
			userId BIGINT,
			replyId BIGINT,
			content text,
			createdAt timestamp,
			updatedAt timestamp,
			upvotes DOUBLE PRECISION DEFAULT 0.0,
			downvotes DOUBLE PRECISION DEFAULT 0.0,
	
			PRIMARY KEY (id)
		);`, year)
		_, e = db.Exec(createCommentsTable)
		DidFail(e, "create comments table")
	}
	createCommentsTable(year)
	createCommentsTable(year + 1)

	createTags := `CREATE TABLE IF NOT EXISTS tags (
		id BIGSERIAL,
		name TEXT NOT NULL,
		upvotes DOUBLE PRECISION DEFAULT 0.0,
		downvotes DOUBLE PRECISION DEFAULT 0.0,
		updatedAt TIMESTAMP DEFAULT now(),
		location TEXT[] NOT NULL,

		PRIMARY KEY (name, location)
	);`
	_, e = db.Exec(createTags)
	DidFail(e, "create tags table")

	createDateFraction := `
	CREATE OR REPLACE FUNCTION dateFrac(beginDate TIMESTAMP, endDate TIMESTAMP, period DOUBLE PRECISION) RETURNS DOUBLE PRECISION AS $$
	BEGIN
		RETURN LEAST(TRUNC(EXTRACT(EPOCH FROM endDate)) - TRUNC(EXTRACT(EPOCH FROM beginDate)), period) / period;
	END;
	$$ LANGUAGE plpgsql`
	_, e = db.Exec(createDateFraction)
	DidFail(e, "create dateFrac function")

	createCoolingFraction := `
	CREATE OR REPLACE FUNCTION coolDown(value DOUBLE PRECISION, beginDate TIMESTAMP, endDate TIMESTAMP, period DOUBLE PRECISION) RETURNS DOUBLE PRECISION AS $$
	BEGIN
		RETURN (1 / (1000 ^ dateFrac(beginDate, endDate, period))) * value;
	END;
	$$ LANGUAGE plpgsql`
	_, e = db.Exec(createCoolingFraction)
	DidFail(e, "create dateFrac function")

	createRatio := `
	CREATE OR REPLACE FUNCTION ratio(x DOUBLE PRECISION, y DOUBLE PRECISION) RETURNS DOUBLE PRECISION AS $$
	BEGIN
		RETURN COALESCE(x / NULLIF(x + y, 0), 0.0);
	END;
	$$ LANGUAGE plpgsql`
	_, e = db.Exec(createRatio)
	DidFail(e, "create ratio function")
}

func DBDeleteTable(db *sql.DB, name string) {
	query := fmt.Sprintf(`DROP TABLE %s`, name)
	_, e := db.Exec(query)
	DidFail(e, "delete table ", name)
}

func DBDeleteAllPosts(db *sql.DB) {
	tables := DBGetTableNamesLike(db, "posts%")
	for _, name := range tables {
		DBDeleteTable(db, name)
	}
}

func DBDeleteAllComments(db *sql.DB) {
	tables := DBGetTableNamesLike(db, "comments%")
	for _, name := range tables {
		DBDeleteTable(db, name)
	}
}

func DBDeleteAllUsers(db *sql.DB) {
	query := `SELECT id FROM users`
	rows, e := db.Query(query)
	if DidFail(e, "get all posts") {
		return
	}
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		DBDeleteTable(db, fmt.Sprintf("User%dPref", id))
		DBDeleteTable(db, fmt.Sprintf("User%dCont", id))
	}
	DBDeleteTable(db, "users")
}

func DBClearAllTables(db *sql.DB) {
	DBDeleteAllPosts(db)
	DBDeleteAllUsers(db)
	DBDeleteAllComments(db)
	DBDeleteTable(db, "tags")
}

func DBGetTableNamesLike(db *sql.DB, query string) []string {
	cmd := fmt.Sprintf("SELECT tablename FROM pg_tables WHERE schemaname='public' AND tablename LIKE '%s'", query)
	rows, e := db.Query(cmd)
	result := []string{}
	if DidFail(e, "get all table names like ", query) {
		return result
	}
	for rows.Next() {
		name := ""
		rows.Scan(&name)
		result = append(result, name)
	}
	return result
}

func BuildUnionForYears(query string, years []int64) string {
	names := []string{}
	for _, y := range years {
		names = append(names, fmt.Sprint(y))
	}
	return BuildUnionForNames(query, names)
}

func BuildUnionForNames(query string, names []string) string {
	result := ""
	for i, y := range names {
		result += strings.Replace(query, "{}", fmt.Sprint(y), 1)
		if i < len(names)-1 {
			result += "\nunion\n"
		}
	}
	return result
}
