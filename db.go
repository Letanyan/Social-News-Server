package main

import (
	"database/sql"
	"fmt"
)

func DBSetup(db *sql.DB) {
	createUsers := `CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL,
		name VARCHAR(21) NOT NULL,
		email text NOT NULL,
		password text NOT NULL,
		registerDate timestamp DEFAULT now(),
		upvotes DOUBLE PRECISION DEFAULT 0.0,
		downvotes DOUBLE PRECISION DEFAULT 0.0,

		PRIMARY KEY (id)
	);`
	_, e := db.Exec(createUsers)
	DidFail(e, "create users table")

	createPosts := `CREATE TABLE IF NOT EXISTS posts (
		id BIGSERIAL,
		userId BIGINT,
		content text,
		tags text[],
		createdAt timestamp,
		updatedAt timestamp,
		upvotes DOUBLE PRECISION DEFAULT 0.0,
		downvotes DOUBLE PRECISION DEFAULT 0.0, 

		PRIMARY KEY (id)
	);`
	_, e = db.Exec(createPosts)
	DidFail(e, "create posts table")

	createTags := `CREATE TABLE IF NOT EXISTS tags (
		id BIGSERIAL,
		name TEXT NOT NULL,
		upvotes DOUBLE PRECISION DEFAULT 0.0,
		downvotes DOUBLE PRECISION DEFAULT 0.0,
		updatedAt TIMESTAMP,

		PRIMARY KEY (name)
	);`
	_, e = db.Exec(createTags)
	DidFail(e, "create tags table")

	createDateFraction := `
	CREATE OR REPLACE FUNCTION date_frac(beginDate TIMESTAMP, endDate TIMESTAMP, duration DOUBLE PRECISION) RETURNS DOUBLE PRECISION AS $$
	BEGIN
		RETURN 1 - LEAST(TRUNC(EXTRACT(EPOCH FROM endDate)) - TRUNC(EXTRACT(EPOCH FROM beginDate)), duration) / duration;
	END;
	$$ LANGUAGE plpgsql
	`
	_, e = db.Exec(createDateFraction)
	DidFail(e, "create dateFrac function")
}

func DBDeleteTable(db *sql.DB, name string) {
	query := fmt.Sprintf(`DROP TABLE %s`, name)
	_, e := db.Exec(query)
	DidFail(e, "delete table ", name)
}

func DBDeleteAllPosts(db *sql.DB) {
	query := `SELECT id FROM posts`
	rows, e := db.Query(query)
	if DidFail(e, "get all posts") {
		return
	}
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		DBDeleteTable(db, fmt.Sprintf("Post%d", id))
	}
	DBDeleteTable(db, "posts")
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
