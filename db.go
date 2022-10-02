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
		upvotes real DEFAULT 0.0,
		downvotes real DEFAULT 0.0,

		PRIMARY KEY (id)
	);`
	_, e := db.Exec(createUsers)
	Failed("create users table", e)

	createPosts := `CREATE TABLE IF NOT EXISTS posts (
		id BIGSERIAL,
		userId BIGINT,
		content text,
		tags text[],
		createdAt timestamp,
		updatedAt timestamp,
		upvotes real DEFAULT 0.0,
		downvotes real DEFAULT 0.0, 

		PRIMARY KEY (id)
	);`
	_, e = db.Exec(createPosts)
	Failed("create posts table", e)

	createTags := `CREATE TABLE IF NOT EXISTS tags (
		id BIGSERIAL,
		name TEXT NOT NULL,
		upvotes REAL DEFAULT 0.0,
		downvotes REAL DEFAULT 0.0,
		updatedAt TIMESTAMP,

		PRIMARY KEY (name)
	);`
	_, e = db.Exec(createTags)
	Failed("create tags table", e)
}

func DBDeleteTable(db *sql.DB, name string) {
	query := fmt.Sprintf(`DROP TABLE %s`, name)
	_, e := db.Exec(query)
	Failed("delete table "+name, e)
}

func DBDeleteAllPosts(db *sql.DB) {
	query := `SELECT id FROM posts`
	rows, e := db.Query(query)
	if Failed("get all posts", e) {
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
	if Failed("get all posts", e) {
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
