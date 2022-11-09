package main

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"math/rand"
	"net/smtp"
	"text/template"
	"time"
)

type User struct {
	ID            int64
	Name          string
	Email         string
	Password      string
	RegisterDate  time.Time
	Upvotes       int64
	Downvotes     int64
	Credits       int32
	ValidationKey int32

	PublicViews     bool
	PublicReadLater bool
	PublicFollowing bool
	PublicIgnored   bool

	PublicPostVotes    bool
	PublicCommentVotes bool
	PublicUserVotes    bool
	PublicTagVotes     bool
}

type UserProfile struct {
	ID           int64
	Name         string
	RegisterDate time.Time
	Upvotes      int64
	Downvotes    int64
}

func SQLFieldsForUser() string {
	return "id, name, email, password, registerDate, upvotes, downvotes, credits, validationKey, " +
		"publicViews, publicReadLater, publicFollowing, publicIgnored, " +
		"publicPostVotes, publicCommentVotes, publicUserVotes, publicTagVotes"
}

func SQLFieldsForUserProfile() string {
	return "p.id, p.name, p.registerDate, p.upvotes, p.downvotes"
}

func SQLFieldsForUserProfileAlias() string {
	return "p.id, p.name, p.registerDate, p.upvotes AS item_up, p.downvotes AS item_down"
}

func ScanUser(row *sql.Row) (User, error) {
	u := User{}
	e := row.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.RegisterDate, &u.Upvotes,
		&u.Downvotes, &u.Credits, &u.ValidationKey,
		&u.PublicViews, &u.PublicReadLater, &u.PublicFollowing, &u.PublicIgnored,
		&u.PublicPostVotes, &u.PublicCommentVotes, &u.PublicTagVotes, &u.PublicTagVotes)
	return u, e
}

func ScanUserProfile(row *sql.Row) (UserProfile, error) {
	u := UserProfile{}
	e := row.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes)
	return u, e
}

func ScanUserProfiles(rows *sql.Rows, includeScore bool, hasVotes bool, hasRank bool) []UserProfile {
	result := []UserProfile{}
	var e error
	for rows.Next() {
		u := UserProfile{}
		var score float64
		var cred float64
		var up int64
		var down int64
		var rank float64
		if hasRank {
			if hasVotes {
				if includeScore {
					e = rows.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &up, &down, &cred, &score, &rank)
				} else {
					e = rows.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &up, &down, &rank)
				}
			} else {
				if includeScore {
					e = rows.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &cred, &score, &rank)
				} else {
					e = rows.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &rank)
				}
			}
		} else {
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
		}

		if DidFail(e, "get user from email/id") {
			continue
		}
		result = append(result, u)
	}
	return result
}

type UserValidationError int

func validateLength(name string, value string, min int, max int) string {
	if len(value) > max {
		return fmt.Sprintf("%s must be at most %d characters long", name, max)
	}
	if len(value) < min {
		return fmt.Sprintf("%s must be at least %d characters long", name, min)
	}
	return ""
}

func DBContainsEmail(db *sql.DB, email string) bool {
	isUsed := `SELECT email FROM users WHERE email = $1`
	res, e := db.Query(isUsed, email)
	if DidFail(e, "get matching email") {
		return true
	}
	var found string
	for res.Next() {
		res.Scan(&found)
	}
	return len(found) > 0
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
	vKey := rand.Int31()
	row := db.QueryRow(insertUser, name, email, DBHashPassword(password), vKey)
	user, e := ScanUser(row)
	if DidFail(e, "create and get user") {
		return User{}
	}

	return user
}

func DBUpdateUser(db *sql.DB, id int64, name string) {
	update := `UPDATE Users SET Name=$1 WHERE id=$2`
	_, e := db.Exec(update, name, id)
	if DidFail(e, "update user name") {
		return
	}
}

func DBValidateUser(db *sql.DB, userId int64, key int32) bool {
	validate := `UPDATE users SET validationKey = 0 WHERE id = $1 AND validationKey = $2 RETURNING id, validationKey`
	row := db.QueryRow(validate, userId, key)
	e := row.Scan(&userId, &key)
	if DidFail(e, "validate user ", userId, " with key ", key) {
		return false
	}
	return key == 0
}

func SendValidationKey(userId int64, email string, key int32) {
	host := "smtp.gmail.com"
	port := "587"
	from := "letanyan.a@gmail.com"
	auth := smtp.PlainAuth("", from, "wlyoihckobjsbzlv", host)
	t, _ := template.ParseFiles("templates/verify.html")
	var body bytes.Buffer
	mimeHeaders := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	body.Write([]byte(fmt.Sprintf("Subject: New Source Email Verification \n%s\n\n", mimeHeaders)))
	t.Execute(&body, struct {
		UserId int64
		Key    int32
	}{
		UserId: userId,
		Key:    key,
	})

	e := smtp.SendMail(host+":"+port, auth, from, []string{email}, body.Bytes())

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
	deleteFromUsers := `UPDATE users SET trashed=true WHERE id=$1`
	_, e := db.Exec(deleteFromUsers, userId)
	DidFail(e, "delete user from users table")

	deleteUserContTable := `UPDATE UserCont SET trashed=true WHERE uid=$1`
	_, e = db.Exec(deleteUserContTable, userId)
	DidFail(e, "delete user content table")
}

// ignore email if userId > 0
func DBGetUser(db *sql.DB, userId int64, email string) User {
	getUser := fmt.Sprintf(`SELECT %s FROM users p WHERE `, SQLFieldsForUser())
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
	if DidFail(e, "get user from email/id ", arg) {
		return User{}
	}

	return user
}

func DBGetUsers(db *sql.DB, popularIn []string, upvotes int64, downvotes int64,
	sortOrder SortOrder, limit int64, offset int64,
	startDate string, endDate string, forUser int64, search string) []UserProfile {
	voteTable := "p"
	if len(popularIn) > 0 {
		voteTable = "v"
	}
	joins := ""
	cond := []string{}

	usingVotesTable := len(popularIn) > 0 || len(startDate) > 0 || len(endDate) > 0

	if usingVotesTable {
		joins = "JOIN Votes v ON v.pid = p.id\n"
		cond = append(cond, "v.kind=0")
	}

	getUsers := SQLGetItems("Users p", voteTable, SQLFieldsForUserProfileAlias(),
		SQLFieldsForUserProfile(), joins, popularIn, cond, usingVotesTable,
		upvotes, downvotes, sortOrder, limit, offset, startDate, endDate, forUser, search)

	rows, e := db.Query(getUsers)
	if DidFail(e, "get users", getUsers) {
		return []UserProfile{}
	}

	result := ScanUserProfiles(rows, true, usingVotesTable, len(search) > 0 && sortOrder == soRank)

	return result
}

func DBUpdatePasswordForUser(db *sql.DB, userId int64, old string, new string) {
	updatePassword := `UPDATE users SET password = $1 WHERE id = $2 AND password = $3`
	hOld := DBHashPassword(old)
	hNew := DBHashPassword(new)
	_, e := db.Exec(updatePassword, hNew, userId, hOld)
	if DidFail(e, "update password") {
		return
	}
}

func DBResetPasswordForUser(db *sql.DB, userId int64, new string) {
	updatePassword := `UPDATE users SET password = $1 WHERE id = $2`
	hNew := DBHashPassword(new)
	_, e := db.Exec(updatePassword, hNew, userId)
	if DidFail(e, "update password") {
		return
	}
}

func DBVoteForUser(db *sql.DB, userId int64, targetId int64, upvoteAmount int64, location []string) (UserProfile, UserPref) {
	updatedField := ""
	isUpvote := upvoteAmount > 0
	if isUpvote {
		updatedField = "upvotes"
	} else {
		updatedField = "downvotes"
		upvoteAmount = -upvoteAmount
	}
	voteQuery := SQLMakeVote(upUser, targetId, -1, location, upvoteAmount, isUpvote)
	updateUser := fmt.Sprintf(`
	%s
	UPDATE users p SET 
	%s = %s + %d,
	credits = credits + 0.75 * %d
	WHERE id = %d
	RETURNING %s`,
		voteQuery,
		updatedField, updatedField, upvoteAmount,
		upvoteAmount, targetId, SQLFieldsForUserProfile())

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

func DBUpdateUserPublicPermissions(db *sql.DB, uid int64, pv bool, prl bool, pi bool, pf bool,
	ppv bool, pcv bool, ptv bool, puv bool) {

	update := fmt.Sprintf(`UPDATE Users 
	SET 
	PublicViews=$1,     
	PublicReadLater=$2, 
	PublicFollowing=$3, 
	PublicIgnored=$4,   
	PublicPostVotes=$5,    
	PublicCommentVotes=$6, 
	PublicUserVotes=$7,    
	PublicTagVotes=$8  
	WHERE id=%d
	`, uid)

	_, e := db.Exec(update, pv, prl, pf, pi, ppv, pcv, puv, ptv)
	if DidFail(e, "update user permissions") {
		return
	}
}
