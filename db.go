package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func DBSetup(db *sql.DB) {
	DBUsersSetup(db)
	DBPostsSetup(db)
	DBCommentsSetup(db)
	DBVotesSetup(db)
	DBTagsSetup(db)
	DBFunctionSetup(db)
}

func DBUsersSetup(db *sql.DB) {
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

	createUserContentTable := `CREATE TABLE IF NOT EXISTS UserCont (
		userId BIGINT NOT NULL,
		postId BIGINT NOT NULL,
		commentId BIGINT NOT NULL,

		PRIMARY KEY (userId, postId, commentId)
	) PARTITION BY HASH(userId);`
	_, e = db.Exec(createUserContentTable)
	DidFail(e, "create user content table")

	// kind (1=user, 2=comment, 3=post, 4=tag)
	createUserPrefTable := `CREATE TABLE IF NOT EXISTS UserPref (
		uid BIGINT NOT NULL,
		kind SMALLINT NOT NULL,
		pid BIGINT NOT NULL,
		sid BIGINT NOT NULL,
		upvotes DOUBLE PRECISION DEFAULT 0.0,
		downvotes DOUBLE PRECISION DEFAULT 0.0,
		updatedAt TIMESTAMP DEFAULT (now() at time zone ('utc')),

		PRIMARY KEY (uid, kind, pid, sid)
	) PARTITION BY HASH(uid);`
	_, e = db.Exec(createUserPrefTable)
	DidFail(e, "create user preference table")

	createUsersTable := func(mod int, rem int) {
		makePrefInstance := fmt.Sprintf(`
		CREATE TABLE UserPref%d 
		PARTITION OF UserPref
		FOR VALUES WITH (modulus %d, remainder %d)
		`, rem, mod, rem)
		_, e := db.Exec(makePrefInstance)
		DidFail(e, "create user pref instance")

		makeContInstance := fmt.Sprintf(`
		CREATE TABLE UserCont%d 
		PARTITION OF UserCont
		FOR VALUES WITH (modulus %d, remainder %d)
		`, rem, mod, rem)
		_, e = db.Exec(makeContInstance)
		DidFail(e, "create user cont instance")
	}
	mod := 20
	for i := 0; i < mod; i += 1 {
		createUsersTable(mod, i)
	}
}

func DBPostsSetup(db *sql.DB) {
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
	_, e := db.Exec(createPosts)
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
}

func DBCommentsSetup(db *sql.DB) {
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
	_, e := db.Exec(createComments)
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
}

func DBVotesSetup(db *sql.DB) {
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
	_, e := db.Exec(createVotes)
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
}

func DBTagsSetup(db *sql.DB) {
	createTags := `CREATE TABLE IF NOT EXISTS tags (
		id BIGSERIAL,
		name TEXT NOT NULL,
		upvotes DOUBLE PRECISION DEFAULT 0.0,
		downvotes DOUBLE PRECISION DEFAULT 0.0,
		updatedAt TIMESTAMP DEFAULT (now() at time zone ('utc')),

		PRIMARY KEY (name)
	);`
	_, e := db.Exec(createTags)
	DidFail(e, "create tags table")
}

func DBFunctionSetup(db *sql.DB) {
	createDateFraction := `
	CREATE OR REPLACE FUNCTION dateFrac(beginDate TIMESTAMP, endDate TIMESTAMP, period DOUBLE PRECISION) RETURNS DOUBLE PRECISION AS $$
	BEGIN
		RETURN LEAST(TRUNC(EXTRACT(EPOCH FROM endDate)) - TRUNC(EXTRACT(EPOCH FROM beginDate)), period) / period;
	END;
	$$ LANGUAGE plpgsql`
	_, e := db.Exec(createDateFraction)
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
	DBDeleteTable(db, "Users")
	DBDeleteTable(db, "UserPref")
	DBDeleteTable(db, "UserCont")
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
