package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func DBSetup(db *sql.DB) {
	createUsers := `CREATE TABLE IF NOT EXISTS Users (
		id BIGSERIAL,
		name VARCHAR(21) NOT NULL,
		email TEXT NOT NULL,
		password TEXT NOT NULL,
		registerDate TIMESTAMP DEFAULT (now() at time zone ('utc')),
		updatedAt TIMESTAMP DEFAULT (now() at time zone ('utc')),
		upvotes DOUBLE PRECISION DEFAULT 0.0,
		downvotes DOUBLE PRECISION DEFAULT 0.0,
		credits INTEGER DEFAULT 25,
		validationKey BIGINT NOT NULL,

		PRIMARY KEY (id)
	);`
	_, e := db.Exec(createUsers)
	DidFail(e, "create users table")

	year := time.Now().UTC().Year()
	createPosts := `CREATE TABLE IF NOT EXISTS Posts (
		id BIGSERIAL NOT NULL,
		userId BIGINT NOT NULL,
		content TEXT,
		tags TEXT[],
		createdAt TIMESTAMP,
		updatedAt TIMESTAMP,
		upvotes DOUBLE PRECISION DEFAULT 0.0,
		downvotes DOUBLE PRECISION DEFAULT 0.0,
		location TEXT[],

		PRIMARY KEY (id, createdAt)
	) PARTITION BY RANGE(createdAt);`
	_, e = db.Exec(createPosts)
	DidFail(e, "create posts table")

	createPostsTable := func(year int) {
		makeInstance := fmt.Sprintf(`
		CREATE TABLE posts%d PARTITION OF posts
		FOR VALUES FROM (TIMESTAMP '%d-01-01' at time zone ('utc')) TO (TIMESTAMP '%d-01-01' at time zone ('utc'));
		CREATE INDEX posts%d_createdAt ON posts%d (id, createdAt);
		`, year, year, year+1, year, year)
		_, e := db.Exec(makeInstance)
		DidFail(e, "create posts instance")
	}
	createPostsTable(year)
	createPostsTable(year + 1)

	createComments := `CREATE TABLE IF NOT EXISTS Comments (
		id BIGSERIAL NOT NULL,
		postId BIGINT,
		userId BIGINT,
		replyId BIGINT,
		content TEXT,
		createdAt TIMESTAMP,
		updatedAt TIMESTAMP,
		upvotes DOUBLE PRECISION DEFAULT 0.0,
		downvotes DOUBLE PRECISION DEFAULT 0.0,

		PRIMARY KEY (id, postId)
	) PARTITION BY HASH(postId);`
	_, e = db.Exec(createComments)
	DidFail(e, "create comments table")
	createCommentsTable := func(mod int, rem int) {
		makeInstance := fmt.Sprintf(`
		CREATE TABLE Comments%d 
		PARTITION OF Comments
		FOR VALUES WITH (modulus %d, remainder %d)
		`, rem, mod, rem)
		_, e := db.Exec(makeInstance)
		DidFail(e, "create posts instance")
	}
	mod := 20
	for i := 0; i < mod; i += 1 {
		createCommentsTable(mod, i)
	}

	createVotes := `CREATE TABLE IF NOT EXISTS Votes (
		kind SMALLINT NOT NULL,
		pid BIGINT NOT NULL,
		sid BIGINT NOT NULL,
		location TEXT[],
		upvotes DOUBLE PRECISION DEFAULT 0.0,
		downvotes DOUBLE PRECISION DEFAULT 0.0,
		updatedAt TIMESTAMP DEFAULT (now() at time zone ('utc')),

		PRIMARY KEY (kind, pid, sid, location)
	) PARTITION BY LIST(kind);`
	_, e = db.Exec(createVotes)
	DidFail(e, "create votes table")
	createVotesTable := func(kind int) {
		makeInstance := fmt.Sprintf(`
		CREATE TABLE Votes%d
		PARTITION OF Votes
		FOR VALUES IN (%d)
		`, kind, kind)
		_, e = db.Exec(makeInstance)
		DidFail(e, "create votes table instance")
	}
	createVotesTable(int(upUser))
	createVotesTable(int(upComment))
	createVotesTable(int(upPost))
	createVotesTable(int(upTag))

	createTags := `CREATE TABLE IF NOT EXISTS tags (
		id BIGSERIAL,
		name TEXT NOT NULL,
		upvotes DOUBLE PRECISION DEFAULT 0.0,
		downvotes DOUBLE PRECISION DEFAULT 0.0,
		updatedAt TIMESTAMP DEFAULT (now() at time zone ('utc')),

		PRIMARY KEY (name)
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

	createScoredRatio := `
	CREATE OR REPLACE FUNCTION scoredRatio(x DOUBLE PRECISION, y DOUBLE PRECISION, beginDate TIMESTAMP, endDate TIMESTAMP, period DOUBLE PRECISION) RETURNS DOUBLE PRECISION AS $$
	BEGIN
		RETURN cooldown(ratio(x, y) * (x - y), beginDate, endDate, period);
	END;
	$$ LANGUAGE plpgsql`
	_, e = db.Exec(createScoredRatio)
	DidFail(e, "create scored ratio function")
}

func DBDeleteTable(db *sql.DB, name string) {
	query := fmt.Sprintf(`DROP TABLE %s`, name)
	_, e := db.Exec(query)
	DidFail(e, "delete table ", name)
}

func DBDeleteAllPosts(db *sql.DB) {
	DBDeleteTable(db, "Posts")
}

func DBDeleteAllComments(db *sql.DB) {
	DBDeleteTable(db, "Comments")
}

func DBDeleteAllVotes(db *sql.DB) {
	DBDeleteTable(db, "Votes")
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
	DBDeleteAllVotes(db)
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
	return BuildUnionForNames(query, "{year}", names)
}

func BuildUnionForNames(query string, placeholder string, names []string) string {
	result := ""
	for i, y := range names {
		result += "(" + strings.ReplaceAll(query, placeholder, fmt.Sprint(y)) + ")"
		if i < len(names)-1 {
			result += "\nunion\n"
		}
	}
	return result
}

func DBCreateTriggers(db *sql.DB) {
	year := utc().Year()

	postsInsertTriggerFunc := fmt.Sprintf(`
	CREATE OR REPLACE FUNCTION posts_insert_trigger()
	RETURNS TRIGGER AS $$
	BEGIN
		INSERT INTO posts%d VALUES (NEW.*);
		RETURN NULL;
	END;
	$$
	LANGUAGE plpgsql;
	`, year)
	_, e := db.Exec(postsInsertTriggerFunc)
	if DidFail(e, "create posts insert trigger function") {
		return
	}

	// postsInsertTrigger := `
	// CREATE TRIGGER insert_posts_trigger
	// BEFORE INSERT ON posts
	// FOR EACH ROW EXECUTE FUNCTION posts_insert_trigger();
	// `
	// _, e = db.Exec(postsInsertTrigger)
	// if DidFail(e, "create post insert trigger") {
	// 	return
	// }
}
