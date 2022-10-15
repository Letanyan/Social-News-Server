package main

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"math/rand"
	"net/smtp"
	"time"
)

type User struct {
	ID            int64
	Name          string
	Email         string
	Password      string
	RegisterDate  time.Time
	Upvotes       float64
	Downvotes     float64
	Credits       int32
	ValidationKey int64
}

type UserProfile struct {
	ID           int64
	Name         string
	RegisterDate time.Time
	Upvotes      float64
	Downvotes    float64
}

func SQLFieldsForUser() string {
	return "id, name, email, password, registerDate, upvotes, downvotes, credits, validationKey"
}

func SQLFieldsForUserProfile() string {
	return "u.id, u.name, u.registerDate, u.upvotes, u.downvotes"
}

func SQLFieldsForUserProfileAlias() string {
	return "u.id, u.name, u.registerDate, u.upvotes AS item_up, u.downvotes AS item_down"
}

func ScanUser(row *sql.Row) (User, error) {
	u := User{}
	e := row.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &u.Credits, &u.ValidationKey)
	return u, e
}

func ScanUserProfile(row *sql.Row) (UserProfile, error) {
	u := UserProfile{}
	e := row.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes)
	return u, e
}

func ScanUserProfiles(rows *sql.Rows, includeScore bool, hasVotes bool) []User {
	result := []User{}
	var e error
	for rows.Next() {
		u := User{}
		var score float64
		var cred float64
		var up float64
		var down float64
		if hasVotes {
			if includeScore {
				e = rows.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &up, &down, &cred, &score)
			} else {
				e = rows.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &up, &down)
			}
		} else {
			if includeScore {
				e = rows.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &cred, &score)
			} else {
				e = rows.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes)
			}
		}
		if DidFail(e, "get user from email/id") {
			continue
		}
		result = append(result, u)
	}
	return result
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
	h := sha256.New()
	h.Write([]byte("{" + password + "_}"))
	result := fmt.Sprintf("%x", h.Sum(nil))
	return result
}

func DBEqualHashAndPassword(hash string, password string) bool {
	return hash == DBHashPassword(password)
}

func DBCreateUser(db *sql.DB, name string, email string, password string) User {
	insertUser := fmt.Sprintf(`INSERT INTO users(name, email, password, validationKey) 
	VALUES ($1, $2, $3, $4) RETURNING %s`, SQLFieldsForUser())
	rand.Seed(utc().UnixNano())
	vKey := rand.Int63()
	row := db.QueryRow(insertUser, name, email, DBHashPassword(password), vKey)
	user, e := ScanUser(row)
	if DidFail(e, "create and get user") {
		return User{}
	}

	return user
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

func DBSignIn(db *sql.DB, email string, password string) User {
	user := DBGetUser(db, 0, email)
	if DBEqualHashAndPassword(user.Password, password) {
		user.Password = ""
		return user
	} else {
		return User{}
	}
}

func DBDeleteUser(db *sql.DB, userId int64) {
	deleteFromUsers := `DELETE FROM users WHERE id=$1`
	_, e := db.Exec(deleteFromUsers, userId)
	DidFail(e, "delete user from users table")

	deleteUserContTable := `DELETE FROM UserPref WHERE uid=$1`
	_, e = db.Exec(deleteUserContTable, userId)
	DidFail(e, "delete user content table")

	deleteUserPrefTable := `DELETE FROM UserPref WHERE userId=$1`
	_, e = db.Exec(deleteUserPrefTable, userId)
	DidFail(e, "delete user preference table")
}

// ignore email if userId > 0
func DBGetUser(db *sql.DB, userId int64, email string) User {
	getUser := fmt.Sprintf(`SELECT %s FROM users u WHERE `, SQLFieldsForUser())
	arg := ""
	if userId > 0 {
		arg = fmt.Sprint(userId)
		getUser += "id = $1"
	} else if len(email) > 0 {
		arg = email
		getUser += "email = $1"
	}
	row := db.QueryRow(getUser, arg)
	user, e := ScanUser(row)
	if DidFail(e, "get user from email/id", arg) {
		return User{}
	}

	return user
}

func DBGetUsers(db *sql.DB, popularIn []string, upvotes int64, downvotes int64,
	sortOrder SortOrder, limit int64, offset int64,
	startDate string, endDate string, forUser int64) []User {
	voteTable := "p"
	if len(popularIn) > 0 {
		voteTable = "v"
	}
	joins := ""
	cond := []string{}

	usingVotesTable := len(popularIn) > 0 || len(startDate) > 0 || len(endDate) > 0

	if usingVotesTable {
		joins = "JOIN Votes v ON v.pid = p.id\n"
		cond = append(cond, "kind=1")
	}

	getUsers := SQLGetItems("users p", voteTable, SQLFieldsForUserProfileAlias(),
		SQLFieldsForUserProfile(), joins, popularIn, cond, usingVotesTable,
		upvotes, downvotes, sortOrder, limit, offset, startDate, endDate, forUser)

	rows, e := db.Query(getUsers)
	if DidFail(e, "get users", getUsers) {
		return []User{}
	}

	result := ScanUserProfiles(rows, true, usingVotesTable)

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

func DBResetPasswordForUser(db *sql.DB, userId int64, new string) {
	updatePassword := "UPDATE users SET password = $1 WHERE id = $2"
	hNew := DBHashPassword(new)
	_, e := db.Exec(updatePassword, hNew, userId)
	if DidFail(e, "update password") {
		return
	}
}

func DBVoteForUser(db *sql.DB, userId int64, targetId int64, upvoteAmount int64, location []string, date string) (UserProfile, UserPref) {
	updatedField := ""
	isUpvote := upvoteAmount > 0
	if isUpvote {
		updatedField = "upvotes"
	} else {
		updatedField = "downvotes"
		upvoteAmount = -upvoteAmount
	}
	voteQuery := SQLMakeVote(upUser, targetId, -1, location, upvoteAmount*sign(isUpvote), date)
	updateUser := fmt.Sprintf(`
	%s
	UPDATE users u SET 
	%s = %s + %d,
	credits = credits + 0.75 * %d
	WHERE id = %d
	RETURNING %s`,
		voteQuery,
		updatedField, updatedField, upvoteAmount,
		upvoteAmount, targetId, SQLFieldsForUserProfile())

	updateUser = ReplaceDateValues(updateUser, date)
	row := db.QueryRow(updateUser)
	user, e := ScanUserProfile(row)
	if DidFail(e, "update user score", targetId) {
		return UserProfile{}, UserPref{}
	}

	pref := DBCreateUserPref(db, userId, upUser, targetId, -1, upvoteAmount*sign(isUpvote))

	return user, pref
}

func DBCanUpdateCredit(db *sql.DB, userId int64, amount int64) bool {
	check := fmt.Sprintf(`SELECT credits FROM Users WHERE id = $1 AND credits >= %d`, amount)
	row := db.QueryRow(check, userId)
	var result int64
	e := row.Scan(&result)
	return e == nil
}

func DBSubtractUserCredit(db *sql.DB, userId int64, amount int64) int64 {
	changeAmount := fmt.Sprintf(`UPDATE Users 
	SET credits = credits - %d 
	WHERE id = $1 AND credits >= %d
	RETURNING credits`, amount, amount)
	row := db.QueryRow(changeAmount, userId)
	var result int64
	e := row.Scan(&result)
	if e != nil {
		return -1
	} else {
		return result
	}
}

func DBAddUserCredit(db *sql.DB, userId int64, amount int64) int64 {
	changeAmount := fmt.Sprintf(`UPDATE Users 
	SET credits = credits + %d
	WHERE id = $1 AND credits < 1000000000000000 - %d
	RETURNING credits`, amount, amount)
	row := db.QueryRow(changeAmount, userId)
	var result int64
	e := row.Scan(&result)
	if e != nil {
		return -1
	} else {
		return result
	}
}
