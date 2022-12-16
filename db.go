package main

import (
	"database/sql"
	"fmt"
)

func DBSetup(db *sql.DB) {
	DBUsersSetup(db)
	DBPostsSetup(db)
	DBCommentsSetup(db)
	DBVotesSetup(db)
	DBLocationSetup(db)
	DBTagsSetup(db)
	DBFlagsSetup(db)
	DBIapSetup(db)
	DBAgentsSetup(db)
	DBUserAuthSetup(db)
	DBPostTagsSetup(db)
	DBMigrations(db)
	DBFunctionSetup(db)
}

func DBUsersSetup(db *sql.DB) {
	createUsers := `CREATE TABLE IF NOT EXISTS Users (
		id BIGSERIAL, -- -1 implies admin user, 0 assumes user does not exist
		name VARCHAR(21) NOT NULL,
		email TEXT NOT NULL, -- empty email implies agent
		password TEXT NOT NULL,
		registerDate TIMESTAMP DEFAULT (now() at time zone 'utc'),
		upvotes BIGINT DEFAULT 0,
		downvotes BIGINT DEFAULT 0,
		credits INTEGER DEFAULT 25,
		validationKey BIGINT NOT NULL,
		loginDate TIMESTAMP DEFAULT (now() at time zone 'utc'),
		streak INTEGER DEFAULT 0,
		trashed BOOLEAN DEFAULT false,
		blocked TIMESTAMP DEFAULT '1970-01-01'::timestamp,
		Investment BIGINT DEFAULT 0,

		publicViews BOOLEAN DEFAULT true,
		publicReadLater BOOLEAN DEFAULT true,
		publicFollowing BOOLEAN DEFAULT true,
		publicIgnored BOOLEAN DEFAULT true,
		publicTagFollow BOOLEAN DEFAULT true,

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

		PRIMARY KEY (uid, pid, sid, kind)
	) PARTITION BY HASH(uid);`
	_, e = db.Exec(createUserContentTable)
	DidFail(e, "create user content table")

	// kind (1=user, 2=comment, 3=post, 4=tag)
	createUserPrefTable := `CREATE TABLE IF NOT EXISTS UserPref (
		uid BIGINT NOT NULL,
		kind SMALLINT NOT NULL,
		pid BIGINT NOT NULL,
		sid BIGINT NOT NULL,
		upvotes BIGINT DEFAULT 0,
		downvotes BIGINT DEFAULT 0,
		updatedOn TIMESTAMP DEFAULT (now() at time zone 'utc'),

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
		upvotes BIGINT DEFAULT 0,
		downvotes BIGINT DEFAULT 0,
		location INT[],
		trashed BOOLEAN DEFAULT false,
		commentCount INTEGER DEFAULT 0,
		edited TIMESTAMP,
		Language TEXT DEFAULT 'english',

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
		upvotes BIGINT DEFAULT 0,
		downvotes BIGINT DEFAULT 0,
		trashed BOOLEAN DEFAULT false,
		replyCount SMALLINT DEFAULT 0,
		isReview BOOLEAN DEFAULT false,
		edited TIMESTAMP,
		Language TEXT DEFAULT 'english',

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
		upvotes BIGINT DEFAULT 0,
		downvotes BIGINT DEFAULT 0,
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

func DBLocationSetup(db *sql.DB) {
	createLocation := `CREATE TABLE IF NOT EXISTS Location (
		id SERIAL,
		name TEXT,

		PRIMARY KEY (id, name)
	);`
	_, e := db.Exec(createLocation)
	DidFail(e, "create location table")
}

func DBTagsSetup(db *sql.DB) {
	createTags := `CREATE TABLE IF NOT EXISTS tags (
		id BIGSERIAL,
		name TEXT NOT NULL,
		upvotes BIGINT DEFAULT 0,
		downvotes BIGINT DEFAULT 0,

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

func DBIapSetup(db *sql.DB) {
	createIap := `CREATE TABLE IF NOT EXISTS Iap (
		userId BIGINT,
		platform CHAR(1),
		productId TEXT,
		data TEXT,

		PRIMARY KEY (userId, platform, productId, data)
	) PARTITION BY HASH(userId);`
	_, e := db.Exec(createIap)
	DidFail(e, "create iap table")
	createIapTable := func(mod int, rem int) {
		makeInstance := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS Iap%d 
		PARTITION OF Iap
		FOR VALUES WITH (modulus %d, remainder %d);
		CREATE INDEX IF NOT EXISTS Iap%d_index 
		ON Iap%d (userId, platform, productId, data)
		`, rem, mod, rem, rem, rem)
		_, e := db.Exec(makeInstance)
		DidFail(e, "create iap partition instance")
	}
	mod := 20
	for i := 0; i < mod; i += 1 {
		createIapTable(mod, i)
	}
}

func DBAgentsSetup(db *sql.DB) {
	createAgents := `CREATE TABLE IF NOT EXISTS Agents (
		agentId BIGINT,
		path TEXT,
		date TIMESTAMP DEFAULT (now() at time zone 'utc'),

		PRIMARY KEY (agentId, path)
	) PARTITION BY HASH(agentId);`
	_, e := db.Exec(createAgents)
	DidFail(e, "create agents table")
	createIapTable := func(mod int, rem int) {
		makeInstance := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS Agents%d 
		PARTITION OF Agents
		FOR VALUES WITH (modulus %d, remainder %d);
		CREATE INDEX IF NOT EXISTS Agents%d_index 
		ON Agents%d (agentId, path)
		`, rem, mod, rem, rem, rem)
		_, e := db.Exec(makeInstance)
		DidFail(e, "create agents partition instance")
	}
	mod := 10
	for i := 0; i < mod; i += 1 {
		createIapTable(mod, i)
	}
}

func DBUserAuthSetup(db *sql.DB) {
	createUserAuth := `CREATE TABLE IF NOT EXISTS UserAuth (
		userId BIGINT,
		deviceId TEXT,
		secret CHAR(8),
		lastAction TIMESTAMP DEFAULT (now() at time zone 'utc'),

		PRIMARY KEY (userId, deviceId)
	) PARTITION BY HASH(userId);`
	_, e := db.Exec(createUserAuth)
	DidFail(e, "create user auth table")
	createUserAuthTable := func(mod int, rem int) {
		makeInstance := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS UserAuth%d 
		PARTITION OF UserAuth
		FOR VALUES WITH (modulus %d, remainder %d);
		CREATE INDEX IF NOT EXISTS UserAuth%d_index 
		ON UserAuth%d (userId, deviceId)
		`, rem, mod, rem, rem, rem)
		_, e := db.Exec(makeInstance)
		DidFail(e, "create user auth partition instance")
	}
	mod := 10
	for i := 0; i < mod; i += 1 {
		createUserAuthTable(mod, i)
	}
}

func DBPostTagsSetup(db *sql.DB) {
	createPostTags := `CREATE TABLE IF NOT EXISTS PostTags (
		postId BIGINT,
		tagId BIGINT,

		PRIMARY KEY (postId, tagId)
	) PARTITION BY HASH(postId);`
	_, e := db.Exec(createPostTags)
	DidFail(e, "create post tags table")
	createPostTagsTable := func(mod int, rem int) {
		makeInstance := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS PostTags%d 
		PARTITION OF PostTags
		FOR VALUES WITH (modulus %d, remainder %d);
		CREATE INDEX IF NOT EXISTS PostTags%d_index 
		ON PostTags%d (postId, tagId)
		`, rem, mod, rem, rem, rem)
		_, e := db.Exec(makeInstance)
		DidFail(e, "create posts tags partition instance")
	}
	mod := 10
	for i := 0; i < mod; i += 1 {
		createPostTagsTable(mod, i)
	}
}

func DBMigrations(db *sql.DB) {
	commands := `
	ALTER TABLE Users 
	ADD COLUMN IF NOT EXISTS PublicTagFollow BOOLEAN 
	DEFAULT true;

	ALTER TABLE Users
	ADD COLUMN IF NOT EXISTS Investment BIGINT
	DEFAULT 0;

	ALTER TABLE Users
	ADD COLUMN IF NOT EXISTS Blocked TIMESTAMP
	DEFAULT '1970-01-01'::timestamp;

	ALTER TABLE Posts
	ADD COLUMN IF NOT EXISTS Language TEXT
	DEFAULT 'english';

	ALTER TABLE Comments
	ADD COLUMN IF NOT EXISTS Language TEXT
	DEFAULT 'english';

	ALTER TABLE Posts DROP COLUMN IF EXISTS ContentVector;
	ALTER TABLE Posts ADD COLUMN IF NOT EXISTS ContentLocaleVector TSVECTOR;
	UPDATE Posts SET ContentLocaleVector = to_tsvector(language::regconfig, content) WHERE ContentLocaleVector is NULL;
	CREATE INDEX IF NOT EXISTS posts_idx_content_vector ON Posts USING gin(ContentLocaleVector);

	ALTER TABLE Comments DROP COLUMN IF EXISTS ContentVector;
	ALTER TABLE Comments ADD COLUMN IF NOT EXISTS ContentLocaleVector TSVECTOR;
	UPDATE Comments SET ContentLocaleVector = to_tsvector(language::regconfig, content) WHERE ContentLocaleVector is NULL;
	CREATE INDEX IF NOT EXISTS comments_idx_content_vector ON Comments USING gin(ContentLocaleVector);

	ALTER Table Posts
	ADD COLUMN IF NOT EXISTS Edited BOOLEAN
	DEFAULT false;

	ALTER Table Comments
	ADD COLUMN IF NOT EXISTS Edited BOOLEAN
	DEFAULT false;

	CREATE SEQUENCE IF NOT EXISTS location_id_seq;
	ALTER TABLE Location ALTER COLUMN id SET NOT NULL;
	ALTER TABLE Location ALTER COLUMN id SET DEFAULT nextval('location_id_seq');
	ALTER SEQUENCE location_id_seq OWNED BY Location.id;

	ALTER TABLE Votes
	ALTER COLUMN Location 
	TYPE INTEGER[] USING Location::int[];

	ALTER TABLE Posts
	ALTER COLUMN Location
	TYPE INTEGER[] USING Location::int[];

	CREATE UNIQUE INDEX IF NOT EXISTS location_name_idx ON Location(name);

	ALTER TABLE Users
	ADD COLUMN IF NOT EXISTS loginDate TIMESTAMP 
	DEFAULT (now() at time zone 'utc');

	ALTER TABLE Users
	ADD COLUMN IF NOT EXISTS streak INTEGER 
	DEFAULT 0;

	ALTER TABLE Posts 
	ALTER COLUMN Edited DROP default;
	ALTER TABLE Posts
	ALTER COLUMN Edited TYPE TIMESTAMP USING createdAt;

	ALTER TABLE Comments 
	ALTER COLUMN Edited DROP default;
	ALTER TABLE Comments
	ALTER COLUMN Edited TYPE TIMESTAMP USING createdAt;

	ALTER TABLE Posts
	ADD COLUMN IF NOT EXISTS FlagCount BIGINT DEFAULT 0;
	ALTER TABLE Comments
	ADD COLUMN IF NOT EXISTS FlagCount BIGINT DEFAULT 0;

	DROP AGGREGATE 
	IF EXISTS scoreValueFactor(DOUBLE PRECISION, DOUBLE PRECISION, DOUBLE PRECISION);
	DROP FUNCTION 
	IF EXISTS scoreValueFactorFinal(DOUBLE PRECISION[]);
	DROP FUNCTION 
	IF EXISTS scoreValueFactorAgg(DOUBLE PRECISION[], DOUBLE PRECISION, DOUBLE PRECISION, DOUBLE PRECISION);
	`
	_, e := db.Exec(commands)
	DidFail(e, "migrations")

	postTagsCount := `SELECT COUNT(*) FROM PostTags;`
	row := db.QueryRow(postTagsCount)
	var count int
	e = row.Scan(&count)
	if DidFail(e, "scan count of PostTags") {
		return
	}
	if count == 0 {
		updatePostTags := `
		INSERT INTO PostTags (postId, tagId)
		SELECT p.id, tagId
		FROM Posts p, unnest(p.tags) tagId
		ON CONFLICT (postId, tagId) 
		DO NOTHING;
		`
		_, e = db.Exec(updatePostTags)
		DidFail(e, "insert post tag ids")
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

	createWeightRatio := `
	CREATE OR REPLACE FUNCTION weightRatio(z DOUBLE PRECISION, x DOUBLE PRECISION, y DOUBLE PRECISION) RETURNS DOUBLE PRECISION AS $$
	BEGIN
		RETURN z * RATIO(x, y);
	END;
	$$ LANGUAGE plpgsql`
	_, e = db.Exec(createWeightRatio)
	DidFail(e, "create weight ratio function")

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

	createInverse := `
	CREATE OR REPLACE FUNCTION InverseNumber(n DOUBLE PRECISION, max DOUBLE PRECISION) 
	RETURNS DOUBLE PRECISION AS $$
	BEGIN
		RETURN LEAST(COALESCE(1 / NULLIF(n, 0), max), max);
	END;
	$$ LANGUAGE plpgsql`
	_, e = db.Exec(createInverse)
	DidFail(e, "create inverse function")

	createScoreValueType := `
	DO
	$$
	BEGIN
		IF NOT EXISTS (
			SELECT * FROM pg_type typ
			INNER JOIN pg_namespace nsp ON nsp.oid = typ.typnamespace
			WHERE 
			nsp.nspname = current_schema() AND 
			typ.typname = 'scorevaluetype'
		) THEN
			CREATE TYPE ScoreValueType AS (
				tagV   DOUBLE PRECISION,
				userV  DOUBLE PRECISION,
				countV DOUBLE PRECISION,
				factor DOUBLE PRECISION
			);
	END IF;
	END;
	$$
	LANGUAGE plpgsql;
	`
	_, e = db.Exec(createScoreValueType)
	DidFail(e, "create score value type")

	createScoreValue := `
	CREATE OR REPLACE FUNCTION scoreValueAgg (cagg ScoreValueType, tagValue DOUBLE PRECISION, userValue DOUBLE PRECISION)
	RETURNS ScoreValueType LANGUAGE plpgsql STRICT AS $$
	BEGIN
		cagg.tagV = cagg.tagV + tagValue;
		cagg.userV = cagg.userV + userValue;
		cagg.countV = cagg.countV + 1;
		cagg.factor = factor;
		RETURN cagg; 
	END; $$; 

	CREATE OR REPLACE FUNCTION scoreValueFinal (cagg ScoreValueType)
	RETURNS DOUBLE PRECISION LANGUAGE plpgsql STRICT AS $$
	BEGIN
		RETURN (cagg.tagV + (cagg.userV / cagg.countV)) * cagg.factor; 
	END; $$;

	-- define user aggregate
	CREATE OR REPLACE AGGREGATE scoreValue (tagValue DOUBLE PRECISION, userValue DOUBLE PRECISION) (
		sfunc = scoreValueAgg,
		stype = ScoreValueType,
		finalfunc = scoreValueFinal,
		initcond = '(0, 0, 0, 0)'
	);`
	_, e = db.Exec(createScoreValue)
	DidFail(e, "create scoreValue aggregate function")

	createSumWeightedRatioScoreValue := `
	CREATE OR REPLACE FUNCTION scoreValueFactorAgg (cagg DOUBLE PRECISION[4], tagValue DOUBLE PRECISION, userValue DOUBLE PRECISION, factor DOUBLE PRECISION)
	RETURNS DOUBLE PRECISION ARRAY[4] LANGUAGE plpgsql STRICT AS $$
	DECLARE nagg DOUBLE PRECISION ARRAY[4]; 
	BEGIN
		nagg[1] = cagg[1] + tagValue;
		nagg[2] = cagg[2] + userValue;
		nagg[3] = cagg[3] + 1;
		nagg[4] = factor;
		RETURN nagg; 
	END; $$; 

	CREATE OR REPLACE FUNCTION scoreValueFactorFinal (cagg DOUBLE PRECISION[4])
	RETURNS DOUBLE PRECISION LANGUAGE plpgsql STRICT AS $$
	BEGIN
		RETURN (cagg[1] + (cagg[2] / cagg[3])) * cagg[4]; 
	END; $$;

	-- define user aggregate
	CREATE OR REPLACE AGGREGATE scoreValueFactor (tagValue DOUBLE PRECISION, userValue DOUBLE PRECISION, factor DOUBLE PRECISION) (
		sfunc = scoreValueFactorAgg,
		stype = DOUBLE PRECISION[4],
		finalfunc = scoreValueFactorFinal,
		initcond = '{0, 0, 0, 0}'
	);`
	_, e = db.Exec(createSumWeightedRatioScoreValue)
	DidFail(e, "create sum weighted ratio score value aggregate function")

	createAggRatioValue := `
	CREATE OR REPLACE FUNCTION sumRatioAgg (accumulator DOUBLE PRECISION, x DOUBLE PRECISION, y DOUBLE PRECISION)
	RETURNS DOUBLE PRECISION LANGUAGE plpgsql STRICT AS $$
	BEGIN
		RETURN accumulator + COALESCE(x / NULLIF(x + y, 0), 0.0);
	END; $$; 

	CREATE OR REPLACE FUNCTION sumRatioFinal (accumulator DOUBLE PRECISION)
	RETURNS DOUBLE PRECISION LANGUAGE plpgsql STRICT AS $$
	BEGIN
		RETURN accumulator; 
	END; $$;

	-- define user aggregate
	CREATE OR REPLACE AGGREGATE sumRatio (x DOUBLE PRECISION, y DOUBLE PRECISION) (
		sfunc = sumRatioAgg,
		stype = DOUBLE PRECISION,
		finalfunc = sumRatioFinal,
		initcond = 0
	);`
	_, e = db.Exec(createAggRatioValue)
	DidFail(e, "create aggregate ratio function")

	createAggWeightRatio := `
	CREATE OR REPLACE FUNCTION sumWeightedRatioAgg (accumulator DOUBLE PRECISION, z DOUBLE PRECISION, x DOUBLE PRECISION, y DOUBLE PRECISION)
	RETURNS DOUBLE PRECISION LANGUAGE plpgsql STRICT AS $$
	BEGIN
		RETURN accumulator + z * COALESCE(x / NULLIF(x + y, 0), 0.0);
	END; $$; 

	CREATE OR REPLACE FUNCTION sumWeightedRatioFinal (accumulator DOUBLE PRECISION)
	RETURNS DOUBLE PRECISION LANGUAGE plpgsql STRICT AS $$
	BEGIN
		RETURN accumulator; 
	END; $$;

	-- define user aggregate
	CREATE OR REPLACE AGGREGATE sumWeightedRatio (z DOUBLE PRECISION, x DOUBLE PRECISION, y DOUBLE PRECISION) (
		sfunc = sumWeightedRatioAgg,
		stype = DOUBLE PRECISION,
		finalfunc = sumWeightedRatioFinal,
		initcond = 0
	);`
	_, e = db.Exec(createAggWeightRatio)
	DidFail(e, "create aggregate weight function")
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
	DBDeleteTable(db, "UserAuth")
}

func DBClearAllTables(db *sql.DB) {
	DBDeleteAllPosts(db)
	DBDeleteAllUsers(db)
	DBDeleteAllVotes(db)
	DBDeleteAllComments(db)
	DBDeleteTable(db, "Tags")
	DBDeleteTable(db, "Iap")
	DBDeleteTable(db, "Agents")
}

func DBClearTrashedContent(db *sql.DB) {
	query := `
	DELETE FROM Posts WHERE Trashed=True;
	DELETE FROM Comments WHERE Trashed=True;
	DELETE FROM Users WHERE Trashed=True;
	DELETE FROM UserCont WHERE Trashed=True;
	`
	_, e := db.Exec(query)
	DidFail(e, "remove trashed content")
}
