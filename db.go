package main

import (
	"database/sql"
	"fmt"
	"strings"
)

func DBSetup(db *sql.DB) {
	DBUsersSetup(db)
	DBPostsSetup(db)
	DBCommentsSetup(db)
	DBVotesSetup(db)
	DBTagsSetup(db)
	DBFlagsSetup(db)
	DBFunctionSetup(db)
}

func DBUsersSetup(db *sql.DB) {
	createUsers := `CREATE TABLE IF NOT EXISTS Users (
		id BIGSERIAL,
		name VARCHAR(21) NOT NULL,
		email TEXT NOT NULL,
		password TEXT NOT NULL,
		registerDate TIMESTAMP DEFAULT (now() at time zone 'utc'),
		upvotes DOUBLE PRECISION DEFAULT 0.0,
		downvotes DOUBLE PRECISION DEFAULT 0.0,
		credits INTEGER DEFAULT 25,
		validationKey BIGINT NOT NULL,
		trashed BOOLEAN DEFAULT false,

		publicViews BOOLEAN DEFAULT true,
		publicReadLater BOOLEAN DEFAULT true,
		publicFollowing BOOLEAN DEFAULT true,
		publicIgnored BOOLEAN DEFAULT true,
		
		publicPostVotes BOOLEAN DEFAULT true,
		publicCommentVotes BOOLEAN DEFAULT true,
		publicUserVotes BOOLEAN DEFAULT true,
		publicTagVotes BOOLEAN DEFAULT true,

		PRIMARY KEY (id)
	);`
	_, e := db.Exec(createUsers)
	DidFail(e, "create users table")

	// FIXME: add created at and updated at dates for UserCont and UserPref tables
	createUserContentTable := `CREATE TABLE IF NOT EXISTS UserCont (
		uid BIGINT NOT NULL,
		pid BIGINT NOT NULL,
		sid BIGINT NOT NULL,
		trashed BOOLEAN DEFAULT false,
		kind SMALLINT DEFAULT 0,
		addedOn TIMESTAMP DEFAULT (now() at time zone 'utc'),

		PRIMARY KEY (uid, pid, sid)
	) PARTITION BY HASH(uid);`
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

		PRIMARY KEY (uid, kind, pid, sid)
	) PARTITION BY HASH(uid);`
	_, e = db.Exec(createUserPrefTable)
	DidFail(e, "create user preference table")

	createUsersTable := func(mod int, rem int) {
		makePrefInstance := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS UserPref%d 
		PARTITION OF UserPref
		FOR VALUES WITH (modulus %d, remainder %d);
		CREATE INDEX IF NOT EXISTS UserPref%d_index ON UserPref%d (uid, kind, pid, sid)
		`, rem, mod, rem, rem, rem)
		_, e := db.Exec(makePrefInstance)
		DidFail(e, "create user pref instance")

		makeContInstance := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS UserCont%d 
		PARTITION OF UserCont
		FOR VALUES WITH (modulus %d, remainder %d);
		CREATE INDEX IF NOT EXISTS UserCont%d_index ON UserCont%d (uid, kind, pid, sid)
		`, rem, mod, rem, rem, rem)
		_, e = db.Exec(makeContInstance)
		DidFail(e, "create user cont instance")
	}
	mod := 20
	for i := 0; i < mod; i += 1 {
		createUsersTable(mod, i)
	}
}

func DBPostsSetup(db *sql.DB) {
	year := utc().Year()
	createPosts := `CREATE TABLE IF NOT EXISTS Posts (
		id BIGSERIAL NOT NULL,
		userId BIGINT NOT NULL,
		content TEXT,
		tags BIGINT[],
		createdAt TIMESTAMP,
		upvotes DOUBLE PRECISION DEFAULT 0.0,
		downvotes DOUBLE PRECISION DEFAULT 0.0,
		location TEXT[],
		trashed BOOLEAN DEFAULT false,
		commentCount INTEGER DEFAULT 0,

		PRIMARY KEY (id, createdAt)
	) PARTITION BY RANGE(createdAt);`
	_, e := db.Exec(createPosts)
	DidFail(e, "create posts table")

	createPostsPartitionTable(db, year)
	createPostsPartitionTable(db, year+1)
}

func createPostsPartitionTable(db *sql.DB, year int) {
	makeInstance := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS posts%d 
	PARTITION OF posts
	FOR VALUES FROM (TIMESTAMP '%d-01-01' at time zone 'utc') TO (TIMESTAMP '%d-01-01' at time zone 'utc');
	CREATE INDEX IF NOT EXISTS posts%d_index ON posts%d (id, createdAt);
	`, year, year, year+1, year, year)
	_, e := db.Exec(makeInstance)
	DidFail(e, "create posts instance")
}

func DBCommentsSetup(db *sql.DB) {
	createComments := `CREATE TABLE IF NOT EXISTS Comments (
		id BIGSERIAL NOT NULL,
		postId BIGINT,
		userId BIGINT,
		replyId BIGINT,
		content TEXT,
		createdAt TIMESTAMP,
		upvotes DOUBLE PRECISION DEFAULT 0.0,
		downvotes DOUBLE PRECISION DEFAULT 0.0,
		trashed BOOLEAN DEFAULT false,
		replyCount SMALLINT DEFAULT 0,

		PRIMARY KEY (id, postId)
	) PARTITION BY HASH(postId);`
	_, e := db.Exec(createComments)
	DidFail(e, "create comments table")
	createCommentsTable := func(mod int, rem int) {
		makeInstance := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS Comments%d 
		PARTITION OF Comments
		FOR VALUES WITH (modulus %d, remainder %d);
		CREATE INDEX IF NOT EXISTS Comments%d_index ON Comments%d (id, postId)
		`, rem, mod, rem, rem, rem)
		_, e := db.Exec(makeInstance)
		DidFail(e, "create posts instance")
	}
	mod := 20
	for i := 0; i < mod; i += 1 {
		createCommentsTable(mod, i)
	}
	addCol := `ALTER TABLE Comments ADD COLUMN IF NOT EXISTS replyCount SMALLINT DEFAULT 0`
	_, e = db.Exec(addCol)
	DidFail(e, "add replyCount col to comments")
}

func DBVotesSetup(db *sql.DB) {
	year := utc().Year()
	createVotes := `CREATE TABLE IF NOT EXISTS Votes (
		kind SMALLINT NOT NULL,
		pid BIGINT NOT NULL,
		sid BIGINT NOT NULL,
		location TEXT[],
		upvotes DOUBLE PRECISION DEFAULT 0.0,
		downvotes DOUBLE PRECISION DEFAULT 0.0,
		updatedAt DATE DEFAULT (now() at time zone 'utc'),

		PRIMARY KEY (kind, pid, sid, location, updatedAt)
	) PARTITION BY RANGE(updatedAt);`
	_, e := db.Exec(createVotes)
	DidFail(e, "create votes table")

	DBCreateVotesPartitionTable(db, year)
	DBCreateVotesPartitionTable(db, year+1)
}

func DBCreateVotesPartitionTable(db *sql.DB, year int) {
	tableName := fmt.Sprintf("Votes%d", year)
	createVotesTable := func() {
		makeInstance := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s
		PARTITION OF Votes
		FOR VALUES FROM ('%d-01-01') TO ('%d-01-01')
		PARTITION BY LIST(kind);
		CREATE INDEX IF NOT EXISTS %s_index ON %s (kind, pid, sid, location, updatedAt)
		`, tableName, year, year+1, tableName, tableName)
		_, e := db.Exec(makeInstance)
		DidFail(e, "create votes table instance")
	}
	createVotesKindTable := func(kind int) {
		kindTableName := fmt.Sprintf("%sk%d", tableName, kind)
		makeInstance := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s
		PARTITION OF %s
		FOR VALUES IN (%d);
		CREATE INDEX IF NOT EXISTS %s_index ON %s (kind, pid, sid, location, updatedAt)
		`, kindTableName, tableName, kind, kindTableName, tableName)
		_, e := db.Exec(makeInstance)
		DidFail(e, "create votes table instance")
	}
	createVotesTable()
	createVotesKindTable(int(upUser))
	createVotesKindTable(int(upComment))
	createVotesKindTable(int(upPost))
	createVotesKindTable(int(upTag))
}

func DBTagsSetup(db *sql.DB) {
	createTags := `CREATE TABLE IF NOT EXISTS tags (
		id BIGSERIAL,
		name TEXT NOT NULL,
		upvotes DOUBLE PRECISION DEFAULT 0.0,
		downvotes DOUBLE PRECISION DEFAULT 0.0,

		PRIMARY KEY (name)
	);`
	_, e := db.Exec(createTags)
	DidFail(e, "create tags table")
}

func DBFlagsSetup(db *sql.DB) {
	createFlags := `CREATE TABLE IF NOT EXISTS Flags (
		id BIGSERIAL,
		uid BIGINT,
		pid BIGINT,
		sid BIGINT,
		kind SMALLINT,
		reason TEXT,
		createdAt TIMESTAMP DEFAULT (now() at time zone 'utc'),

		PRIMARY KEY (id, pid)
	) PARTITION BY HASH(pid);`
	_, e := db.Exec(createFlags)
	DidFail(e, "create comments table")
	createFlagsTable := func(mod int, rem int) {
		makeInstance := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS Flags%d 
		PARTITION OF Flags
		FOR VALUES WITH (modulus %d, remainder %d);
		CREATE INDEX IF NOT EXISTS Flags%d_index ON Flags%d (id, pid)
		`, rem, mod, rem, rem, rem)
		_, e := db.Exec(makeInstance)
		DidFail(e, "create posts instance")
	}
	mod := 20
	for i := 0; i < mod; i += 1 {
		createFlagsTable(mod, i)
	}
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

	createDepreciationOverTime := `
	CREATE OR REPLACE FUNCTION depreciateValue(n DOUBLE PRECISION, beginDate TIMESTAMP, endDate TIMESTAMP) RETURNS DOUBLE PRECISION AS $$
	BEGIN
		RETURN 0.99 ^ (n * 229.105288 / EXTRACT(DAYS FROM (endDate - beginDate)));
	END;
	$$ LANGUAGE plpgsql`
	_, e = db.Exec(createDepreciationOverTime)
	DidFail(e, "create deprecation over time function")
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
