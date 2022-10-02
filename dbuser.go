package main

import (
	"database/sql"
	"fmt"
)

func DBVerifyUserName(name string) bool {
	return len(name) <= 21 && len(name) > 0
}

func DBIsValidEmail(db *sql.DB, email string) bool {
	isUsed := `SELECT email FROM users WHERE email = $1`
	res := db.QueryRow(isUsed, email)
	var found string
	e := res.Scan(&found)
	return e != nil
}

func DBHashPassword(password string) string {
	return password + "salt"
}

func DBCreateUser(db *sql.DB, name string, email string, password string) {
	insertUser := `INSERT INTO users(name, email, password) VALUES ($1, $2, $3)`
	_, e := db.Exec(insertUser, name, email, password)
	if Failed("insert user", e) {
		return
	}

	getUserId := `SELECT id FROM users WHERE email=$1`
	row := db.QueryRow(getUserId, email)
	var userId int64
	e = row.Scan(&userId)
	if Failed("get user ID", e) {
		return
	}

	createUserContentTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS User%dCont (
		postId BIGINT NOT NULL,
		commentId BIGINT NOT NULL,

		PRIMARY KEY (postId, commentId)
	);`, userId)
	_, e = db.Exec(createUserContentTable)
	if Failed("create user content table for user "+fmt.Sprint(userId), e) {
		return
	}

	// kind (1=user, 2=comment, 3=post, 4=tag)
	createUserPrefTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS User%dPref (
		kind SMALLINT NOT NULL,
		pid BIGINT NOT NULL,
		sid BIGINT NOT NULL,
		upvotes REAL DEFAULT 0.0,
		downvotes REAL DEFAULT 0.0,

		PRIMARY KEY (kind, pid, sid)
	);`, userId)
	_, e = db.Exec(createUserPrefTable)
	if Failed("create user preference table for user "+fmt.Sprint(userId), e) {
		return
	}
}

func DBDeleteUser(db *sql.DB, userId int) {
	deleteFromUsers := `DELETE FROM users WHERE id=$1`
	_, e := db.Exec(deleteFromUsers, userId)
	Failed("delete user from users table", e)

	deleteUserContTable := fmt.Sprintf(`DROP TABLE User%dCont`, userId)
	_, e = db.Exec(deleteUserContTable)
	Failed("delete user content table", e)

	deleteUserPrefTable := fmt.Sprintf(`DROP TABLE User%dPref`, userId)
	_, e = db.Exec(deleteUserPrefTable)
	Failed("delete user preference table", e)
}

func DBVoteForUser(db *sql.DB, userId int64, sourceId int64, isUpvote bool) {
	updatedField := ""
	if isUpvote {
		updatedField = "upvotes"
	} else {
		updatedField = "downvotes"
	}
	vote := fmt.Sprintf(`INSERT INTO User%dPref (kind, pid, sid)
	VALUES (1, %d, -1) ON CONFLICT (kind, pid, sid) DO NOTHING;
	UPDATE User%dPref 
	SET %s = %s + 1
	WHERE kind=1 AND pid = %d
	`, userId, sourceId, userId, updatedField, updatedField, sourceId)
	_, e := db.Query(vote)
	Failed("vote for user", e)
}
