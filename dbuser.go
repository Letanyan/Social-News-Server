package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"net/smtp"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID            int64
	Name          string
	Email         string
	Password      string
	RegisterDate  time.Time
	UpdatedAt     time.Time
	Upvotes       float64
	Downvotes     float64
	ValidationKey int64
}

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
	result, e := bcrypt.GenerateFromPassword([]byte(password), 7)
	if DidFail(e, "hash password") {
		return ""
	}
	return string(result)
}

func DBCompareHashAndPassword(hash string, password string) bool {
	e := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return !DidFail(e, "compare hash and password")
}

func DBCreateUser(db *sql.DB, name string, email string, password string) User {
	insertUser := `INSERT INTO users(name, email, password, validationKey) 
	VALUES ($1, $2, $3, $4) RETURNING id, name, email, password, registerDate, updatedAt, upvotes, downvotes, validationKey`
	rand.Seed(utc().UnixNano())
	vKey := rand.Int63()
	row := db.QueryRow(insertUser, name, email, DBHashPassword(password), vKey)
	var userId int64
	var reg time.Time
	var upt time.Time
	var upv float64
	var dwn float64
	e := row.Scan(&userId, &name, &email, &password, &reg, &upt, &upv, &dwn, &vKey)
	if DidFail(e, "create and get user") {
		return User{}
	}

	createUserContentTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS User%dCont (
		postId BIGINT NOT NULL,
		commentId BIGINT NOT NULL,

		PRIMARY KEY (postId, commentId)
	);`, userId)
	_, e = db.Exec(createUserContentTable)
	if DidFail(e, "create user content table for user ", userId) {
		return User{}
	}

	// kind (1=user, 2=comment, 3=post, 4=tag)
	createUserPrefTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS User%dPref (
		kind SMALLINT NOT NULL,
		pid BIGINT NOT NULL,
		sid BIGINT NOT NULL,
		upvotes DOUBLE PRECISION DEFAULT 0.0,
		downvotes DOUBLE PRECISION DEFAULT 0.0,
		updatedAt TIMESTAMP,

		PRIMARY KEY (kind, pid, sid)
	);`, userId)
	_, e = db.Exec(createUserPrefTable)
	if DidFail(e, "create user preference table for user ", userId) {
		return User{}
	}

	return User{userId, name, email, password, reg, upt, upv, dwn, vKey}
}

func DBValidateUser(db *sql.DB, userId int64, key int64) bool {
	validate := `UPDATE users SET validationKey = 0 WHERE id = $1 AND validationKey = $2 RETURNING id, validationKey`
	row := db.QueryRow(validate, userId, key)
	e := row.Scan(&userId, &key)
	if DidFail(e, "validate user ", userId, " with key ", key) {
		return false
	}
	return key == 0
}

func SendValidationKey(userId int64, email string, key int64) {
	host := "smtp.gmail.com"
	port := "587"
	from := "from email goes here"
	auth := smtp.PlainAuth("", from, "password for the from email", host)
	mess := fmt.Sprintf("to verify your email please click the link https://localhost:8080/verify?id=%d&key=%d", userId, key)
	message := []byte(mess)

	e := smtp.SendMail(host+":"+port, auth, from, []string{email}, message)

	if DidFail(e, "send mail") {
		return
	}
}

func DBDeleteUser(db *sql.DB, userId int64) {
	deleteFromUsers := `DELETE FROM users WHERE id=$1`
	_, e := db.Exec(deleteFromUsers, userId)
	DidFail(e, "delete user from users table")

	deleteUserContTable := fmt.Sprintf(`DROP TABLE User%dCont`, userId)
	_, e = db.Exec(deleteUserContTable)
	DidFail(e, "delete user content table")

	deleteUserPrefTable := fmt.Sprintf(`DROP TABLE User%dPref`, userId)
	_, e = db.Exec(deleteUserPrefTable)
	DidFail(e, "delete user preference table")
}

// ignore email if userId > 0
func DBGetUser(db *sql.DB, userId int64, email string) User {
	getUser := `SELECT id, name, email, password, registerDate, updatedAt, upvotes, downvotes, validationKey FROM users WHERE `
	arg := ""
	if userId > 0 {
		arg = fmt.Sprint(userId)
		getUser += "id = $1"
	} else if len(email) > 0 {
		arg = email
		getUser += "email = $1"
	}
	row := db.QueryRow(getUser, arg)
	name := ""
	password := ""
	var reg time.Time
	var upt time.Time
	var upv float64
	var dwn float64
	var vKey int64
	e := row.Scan(&userId, &name, &email, &password, &reg, &upt, &upv, &dwn, &vKey)
	if DidFail(e, "get user from email/id", arg) {
		return User{}
	}

	return User{userId, name, email, password, reg, upt, upv, dwn, vKey}
}

func DBGetUsers(db *sql.DB, upvotes int64, downvotes int64, sortOrder SortOrder, limit int64, offset int64) []User {
	getUsers := `SELECT id, name, upvotes, downvotes, RATIO(upvotes, downvotes) AS cred, upvotes * RATIO(upvotes, downvotes) AS score  
	FROM users`

	upClause := ""
	if upvotes > 0 {
		upClause = fmt.Sprintf("upvotes > %d", upvotes)
	} else if upvotes < 0 {
		upClause = fmt.Sprintf("upvotes < %d", -upvotes)
	}
	downClause := ""
	if downvotes > 0 {
		downClause = fmt.Sprintf("downvotes > %d", downvotes)
	} else if upvotes < 0 {
		downClause = fmt.Sprintf("downvotes < %d", -downvotes)
	}
	voteCondition := ""
	if len(upClause) > 0 && len(downClause) > 0 {
		voteCondition = upClause + " AND " + downClause
	} else if len(upClause) > 0 {
		voteCondition = upClause
	} else if len(downClause) > 0 {
		voteCondition = downClause
	}

	if len(voteCondition) > 0 {
		getUsers += "WHERE " + voteCondition + "\n"
	}

	getUsers += SQLSortOrder(sortOrder)
	getUsers += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	row, e := db.Query(getUsers)
	if DidFail(e, "get users") {
		return []User{}
	}

	name := ""
	var userId int64
	var upv float64
	var dwn float64
	result := []User{}
	for row.Next() {
		e := row.Scan(&userId, &name, &upv, &dwn)
		if DidFail(e, "get user from email/id") {
			continue
		}
		u := User{ID: userId, Name: name, Upvotes: upv, Downvotes: dwn}
		result = append(result, u)
	}

	return result
}

func DBUpdatePasswordForUser(db *sql.DB, userId int64, old string, new string) {
	updatePassword := "UPDATE users SET password = $1 WHERE id = $2 AND password = $3"
	hOld := DBHashPassword(old)
	hNew := DBHashPassword(new)
	_, e := db.Exec(updatePassword, hNew, userId, hOld)
	if DidFail(e, "update password") {
		return
	}
}

func DBVoteForUser(db *sql.DB, userId int64, targetId int64, upvoteAmount int64) (User, UserPref) {
	nowTime := formatNow()
	updatedField := ""
	otherField := ""
	isUpvote := upvoteAmount > 0
	if isUpvote {
		updatedField = "upvotes"
		otherField = "downvotes"
	} else {
		updatedField = "downvotes"
		otherField = "upvotes"
		upvoteAmount = -upvoteAmount
	}
	updateUser := fmt.Sprintf(`
	UPDATE users
	SET %s = cooldown(%s, updatedAt, '%s', 31536000) + %d,
	%s = cooldown(%s, updatedAt, '%s', 31536000),
	updatedAt = '%s'
	WHERE id = $1
	RETURNING id, name, email, password, registerDate, updatedAt, upvotes, downvotes, validationKey
	`, updatedField, updatedField, nowTime, upvoteAmount, otherField, otherField, nowTime, nowTime)
	row := db.QueryRow(updateUser, targetId)
	name := ""
	password := ""
	email := ""
	var reg time.Time
	var upt time.Time
	var upv float64
	var dwn float64
	var vKey int64
	e := row.Scan(&targetId, &name, &email, &password, &reg, &upt, &upv, &dwn, &vKey)
	DidFail(e, "update user score", targetId)

	vote := fmt.Sprintf(`INSERT INTO User%dPref (kind, pid, sid)
	VALUES (1, %d, -1) ON CONFLICT (kind, pid, sid) DO NOTHING;
	UPDATE User%dPref 
	SET %s = cooldown(%s, updatedAt, '%s', 31536000) + %d,
	%s = cooldown(%s, updatedAt, '%s', 31536000)
	WHERE kind=1 AND pid = %d
	RETURNING kind, pid, sid, upvotes, downvotes
	`, userId, targetId, userId, updatedField, updatedField, nowTime, upvoteAmount, otherField, otherField, nowTime, targetId)
	row = db.QueryRow(vote)
	pref, e := ScanUserPrefRow(row, false)
	DidFail(e, "vote for user")

	user := User{targetId, name, email, password, reg, upt, upv, dwn, vKey}

	return user, pref
}

type UserPrefKind int

const (
	upUser UserPrefKind = iota + 1
	upComment
	upPost
	upTag
)

type UserPref struct {
	Kind      UserPrefKind
	PID       int64
	SID       int64
	Upvotes   float64
	Downvotes float64
}

func ScanUserPrefRow(row *sql.Row, includeScore bool) (UserPref, error) {
	var kind UserPrefKind
	var pid int64
	var sid int64
	var up float64
	var down float64
	var cred float64
	var score float64
	var e error
	if includeScore {
		e = row.Scan(&kind, &pid, &sid, &up, &down, &cred, &score)
	} else {
		e = row.Scan(&kind, &pid, &sid, &up, &down)
	}
	return UserPref{kind, pid, sid, up, down}, e
}

func ScanUserPrefRows(rows *sql.Rows, includeScore bool) ([]UserPref, error) {
	result := []UserPref{}
	var kind UserPrefKind
	var pid int64
	var sid int64
	var up float64
	var down float64
	var cred float64
	var score float64
	var e error
	for rows.Next() {
		if includeScore {
			e = rows.Scan(&kind, &pid, &sid, &up, &down, &cred, &score)
		} else {
			e = rows.Scan(&kind, &pid, &sid, &up, &down)
		}
		result = append(result, UserPref{kind, pid, sid, up, down})
	}
	return result, e
}

// ignore kind if it equals 0. ignore pid if it equals 0. ignore sid if it equals 0
func DBGetUserPref(db *sql.DB, userId int64, sortOrder SortOrder, kind UserPrefKind, pid int64, sid int64, upvotes int64, downvotes int64, limit int64, offset int64) []UserPref {
	query := fmt.Sprintf(`SELECT kind, pid, sid, upvotes, downvotes, RATIO(upvotes, downvotes) AS cred, upvotes * RATIO(upvotes, downvotes) AS score FROM User%dPref
	`, userId)

	cond := ""
	if kind > 0 {
		cond = fmt.Sprintf(" WHERE kind = %d", kind)
	}
	if pid > 0 {
		if len(cond) == 0 {
			cond += " WHERE "
		} else {
			cond += " AND "
		}
		cond += fmt.Sprintf("pid = %d", pid)
	}
	if sid > 0 {
		if len(cond) == 0 {
			cond += " WHERE "
		} else {
			cond += " AND "
		}
		cond += fmt.Sprintf("sid = %d", sid)
	}
	if upvotes != 0 {
		if len(cond) == 0 {
			cond += " WHERE "
		} else {
			cond += " AND "
		}
		if upvotes > 0 {
			cond += fmt.Sprintf("upvotes > %d", upvotes)
		} else {
			cond += fmt.Sprintf("upvotes < %d", -upvotes)
		}
	}
	if downvotes != 0 {
		if len(cond) == 0 {
			cond += " WHERE "
		} else {
			cond += " AND "
		}
		if downvotes > 0 {
			cond += fmt.Sprintf("downvotes > %d", upvotes)
		} else {
			cond += fmt.Sprintf("downvotes < %d", -upvotes)
		}
	}

	query += cond + "\n"
	query += SQLSortOrder(sortOrder)

	query += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
	rows, e := db.Query(query)
	result := []UserPref{}
	if DidFail(e, "select user prefs for ", userId, " of kind ", kind, " pid:", pid, " sid: ", sid) {
		return result
	}
	result, e = ScanUserPrefRows(rows, true)
	if DidFail(e, "get user pref rows") {
		return result
	}

	return result
}
