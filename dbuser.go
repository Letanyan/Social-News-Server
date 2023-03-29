package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"math/rand"
	"time"
)

type User struct {
	ID            int64 `json:"ID,string"`
	Name          string
	Email         string
	Password      string
	RegisterDate  time.Time
	Upvotes       int64
	Downvotes     int64
	Credits       int32
	Investment    int32
	ValidationKey int32 `json:"ValidationKey,string"`
	LoginDate     time.Time
	Streak        int32
	Blocked       time.Time

	PublicViews     bool
	PublicReadLater bool
	PublicFollowing bool
	PublicIgnored   bool
	PublicTagFollow bool

	PublicPostVotes    bool
	PublicCommentVotes bool
	PublicUserVotes    bool
	PublicTagVotes     bool
}

type UserProfile struct {
	ID           int64 `json:"ID,string"`
	Name         string
	RegisterDate time.Time
	Upvotes      int64
	Downvotes    int64
	Investment   int64
	IsAgent      bool

	Score float64
	Cred  float64
	Rank  float64
}

func SQLFieldsForUser() string {
	return "id, name, email, password, registerDate, upvotes, downvotes, credits, investment, validationKey, " +
		"loginDate, streak, blocked, " +
		"publicViews, publicReadLater, publicFollowing, publicIgnored, publicTagFollow, " +
		"publicPostVotes, publicCommentVotes, publicUserVotes, publicTagVotes"
}

func SQLFieldsForUserProfile() string {
	return "p.id, p.name, p.registerDate, p.upvotes, p.downvotes, p.investment, p.email"
}

func SQLFieldsForUserProfileAlias() string {
	return "p.id, p.name, p.registerDate, p.upvotes AS item_up, p.downvotes AS item_down, p.investment, p.email"
}

func ScanUser(row *sql.Row) (User, error) {
	u := User{}
	e := row.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.RegisterDate, &u.Upvotes,
		&u.Downvotes, &u.Credits, &u.Investment, &u.ValidationKey,
		&u.LoginDate, &u.Streak, &u.Blocked,
		&u.PublicViews, &u.PublicReadLater, &u.PublicFollowing, &u.PublicIgnored, &u.PublicTagFollow,
		&u.PublicPostVotes, &u.PublicCommentVotes, &u.PublicTagVotes, &u.PublicTagVotes)
	return u, e
}

func ScanUsers(rows *sql.Rows) ([]User, error) {
	result := []User{}
	for rows.Next() {
		u := User{}
		e := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.RegisterDate, &u.Upvotes,
			&u.Downvotes, &u.Credits, &u.Investment, &u.ValidationKey,
			&u.LoginDate, &u.Streak, &u.Blocked,
			&u.PublicViews, &u.PublicReadLater, &u.PublicFollowing, &u.PublicIgnored, &u.PublicTagFollow,
			&u.PublicPostVotes, &u.PublicCommentVotes, &u.PublicTagVotes, &u.PublicTagVotes)
		if DidFail(e, "scan user") {
			return result, e
		}
		result = append(result, u)
	}

	return result, nil
}

func ScanUserProfile(row *sql.Row) (UserProfile, error) {
	u := UserProfile{}
	var email string
	e := row.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &u.Investment, &email)
	u.IsAgent = len(email) == 0
	return u, e
}

func ScanUserProfiles(rows *sql.Rows, includeScore bool, hasVotes bool, hasRank bool) []UserProfile {
	result := []UserProfile{}
	var e error
	defer rows.Close()
	for rows.Next() {
		u := UserProfile{}
		var up int64
		var down int64
		var email string
		if hasRank {
			if hasVotes {
				if includeScore {
					e = rows.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &u.Investment, &email, &up, &down, &u.Cred, &u.Score, &u.Rank)
				} else {
					e = rows.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &u.Investment, &email, &up, &down, &u.Rank)
				}
			} else {
				if includeScore {
					e = rows.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &u.Investment, &email, &u.Cred, &u.Score, &u.Rank)
				} else {
					e = rows.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &u.Investment, &email, &u.Rank)
				}
			}
		} else {
			if hasVotes {
				if includeScore {
					e = rows.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &u.Investment, &email, &up, &down, &u.Cred, &u.Score)
				} else {
					e = rows.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &u.Investment, &email, &up, &down)
				}
			} else {
				if includeScore {
					e = rows.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &u.Investment, &email, &u.Cred, &u.Score)
				} else {
					e = rows.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &u.Investment, &email)
				}
			}
		}
		u.IsAgent = len(email) == 0

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
	rows, e := db.Query(isUsed, email)
	if DidFail(e, "get matching email") {
		return true
	}
	defer rows.Close()
	var found string
	for rows.Next() {
		rows.Scan(&found)
	}
	return len(found) > 0
}

func DBHashPassword(password string) string {
	h := sha256.New()
	h.Write([]byte("{" + password + "_}"))
	result := base64.StdEncoding.EncodeToString(h.Sum(nil))
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

func DBCreateTempUser(db *sql.DB, deviceId string) User {
	removeOld := fmt.Sprintf(`
	SELECT %s
	FROM Users u
	JOIN UserAuth a ON u.id = a.userid
	WHERE
		a.deviceId = $1 AND
		u.email = 'temp@new-source.app'
	`, SQLFieldsForUser())
	rows, e := db.Query(removeOld, DBAlphaNumeric(deviceId))

	if !DidFail(e, "remove old user with device id") {
		users, _ := ScanUsers(rows)
		if len(users) > 0 {
			return users[0]
		}
	}

	user := DBCreateUser(db, "Temporary", "temp@new-source.app", generateRandomString(64))
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
	user, _ := DBGetUser(db, userId, "")
	if user.Email == "temp@new-source.app" {
		return false
	}

	validate := `UPDATE users SET validationKey = 0 WHERE id = $1 AND validationKey = $2 RETURNING id, validationKey`
	row := db.QueryRow(validate, userId, key)
	e := row.Scan(&userId, &key)
	if DidFail(e, "validate user ", userId, " with key ", key) {
		return false
	}
	return key == 0
}

func DBSignIn(db *sql.DB, email string, password string) (User, int32) {
	if len(password) < 6 {
		return User{ID: -2}, 0
	}
	user, streak := DBGetUser(db, 0, email)
	if user.ID == 0 {
		return User{ID: -2}, 0
	} else if DBEqualHashAndPassword(user.Password, password) {
		user.Password = ""
		return user, streak
	} else {
		return User{ID: -3}, 0
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

func DBDeleteTempUser(db *sql.DB, daysAgo int) {
	deleteFromUsers := `
	DELETE FROM UserAuth p
	USING Users u
	WHERE 
		u.email = 'temp@new-source.app' AND 
		u.loginDate < ((now() at time zone 'utc') - interval '%d day') AND
		u.id = p.userId;

	DELETE FROM Users 
	WHERE 
		email='temp@new-source.app' AND 
		loginDate < ((now() at time zone 'utc') - interval '%d day')`
	_, e := db.Exec(deleteFromUsers)
	DidFail(e, "delete user from users table")
}

func DBBlockUser(db *sql.DB, userId int64, duration int) {
	query := fmt.Sprintf(`
	UPDATE Users 
	SET Blocked=(now() at time zone 'utc') + INTERVAL '%d day'
	WHERE id=%d
	`, duration, userId)
	_, e := db.Exec(query)
	DidFail(e, "block user ", userId, " for duration ", duration)
}

func DBGetUserAgent(db *sql.DB, name string) User {
	getUser := fmt.Sprintf(`
	SELECT %s 
	FROM Users p 
	WHERE email='' AND name=$1`, SQLFieldsForUser())
	row := db.QueryRow(getUser, name)
	user, e := ScanUser(row)
	if DidFail(e, "get user agent ", name) {
		return User{}
	}
	return user
}

// ignore email if userId > 0
func DBGetUser(db *sql.DB, userId int64, email string) (User, int32) {
	getUser := fmt.Sprintf(`SELECT %s FROM users p WHERE `, SQLFieldsForUser())
	arg := ""
	if userId > 0 || userId == -1 {
		arg = fmt.Sprint(userId)
		getUser += "id = $1"
	} else if len(email) > 0 {
		arg = email
		getUser += "email = $1"
	} else {
		getUser += "FALSE"
	}
	rows, e := db.Query(getUser, arg)
	if DidFail(e, "get user from email/id ", getUser) {
		return User{}, 0
	}
	users, _ := ScanUsers(rows)
	user := User{}
	if len(users) != 1 {
		return user, 0
	} else {
		user = users[0]
	}

	update, streak := DBUpdateUserLogin(db, user)
	user.Credits += update
	user.Streak = streak

	return user, update
}

func DBGetNextStreakAmount(current int32) int32 {
	sample := []int32{
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
		2, 2, 2, 2, 2, 2, 2, 2,
		3, 3, 3, 3,
		4, 4,
		5,
	}
	i := len(sample)
	r := rand.Intn(i)
	result := sample[r]
	if result == current {
		result += 1
	}
	return result
}

func DBUpdateUserLogin(db *sql.DB, user User) (int32, int32) {
	if len(user.Email) == 0 {
		return 0, 0
	}
	now := utc()
	login := user.LoginDate
	ny, nm, nd := now.Date()
	ly, lm, ld := login.Date()
	streak := user.Streak
	unlock := usersMutex.Lock(fmt.Sprintf("%d", user.ID))
	defer unlock()
	var result int32 = 0
	if ly != ny || lm != nm || ld != nd {
		nextDay := login.AddDate(0, 0, 1)
		if nextDay.Day() == nd && nextDay.Month() == nm && nextDay.Year() == ny {
			result = streak
			streak = DBGetNextStreakAmount(streak)
		} else {
			streak = DBGetNextStreakAmount(streak)
			result = 1
		}
	}
	query := fmt.Sprintf(`
	UPDATE Users
	SET
		loginDate = (now() at time zone 'utc'),
		streak = %d,
		credits = credits + %d
	WHERE
		id = %d
	`, streak, result, user.ID)
	_, e := db.Exec(query)
	if DidFail(e, "update user streak and login date") {
		return 0, 0
	}
	return result, streak
}

func DBGetUsers(db *sql.DB, popularIn []string, upvotes int64, downvotes int64,
	sortOrder SortOrder, limit int64, offset int64,
	startDate string, endDate string, forUser int64, search string) []UserProfile {
	voteTable := "p"
	usingVotesTable := len(popularIn) > 0 || len(startDate) > 0 || len(endDate) > 0
	if usingVotesTable {
		voteTable = "v"
	}
	joins := ""
	cond := []string{}

	if usingVotesTable {
		joins = "JOIN Votes v ON v.pid = p.id\n"
		cond = append(cond, "v.kind=0")
	}

	locArray := ""
	if len(popularIn) > 0 {
		locArray = DBGetLocationIndex(db, popularIn)
	}
	getUsers := SQLGetItems(db, "Users p", voteTable, SQLFieldsForUserProfileAlias(),
		SQLFieldsForUserProfile(), joins, locArray, cond, usingVotesTable,
		upvotes, downvotes, sortOrder, limit, offset, 0, startDate, endDate, forUser, search)

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
	if userId == targetId {
		return UserProfile{}, UserPref{}
	}
	updatedField := ""
	isUpvote := upvoteAmount > 0
	if isUpvote {
		updatedField = "upvotes"
	} else {
		updatedField = "downvotes"
		upvoteAmount = -upvoteAmount
	}
	locIndex := DBCreateLocation(db, location)
	voteQuery := SQLMakeVote(upUser, targetId, -1, locIndex, upvoteAmount, isUpvote)
	updateUser := fmt.Sprintf(`
	%s
	UPDATE users p SET 
	%s = %s + %d
	WHERE id = %d
	RETURNING %s`,
		voteQuery,
		updatedField, updatedField, upvoteAmount,
		targetId,
		SQLFieldsForUserProfile())

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
	changeAmount := fmt.Sprintf(`
	UPDATE Users 
	SET credits = credits - %d,
	investment = investment + %d
	WHERE id = $1 AND credits >= %d
	RETURNING credits`, amount, amount, amount)
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
	ppv bool, pcv bool, ptv bool, puv bool, ptf bool) {

	update := fmt.Sprintf(`UPDATE Users 
	SET 
	PublicViews=$1,     
	PublicReadLater=$2, 
	PublicFollowing=$3, 
	PublicIgnored=$4,   
	PublicPostVotes=$5,    
	PublicCommentVotes=$6, 
	PublicUserVotes=$7,    
	PublicTagVotes=$8,
	PublicTagFollow=$9
	WHERE id=%d
	`, uid)

	_, e := db.Exec(update, pv, prl, pf, pi, ppv, pcv, puv, ptv, ptf)
	if DidFail(e, "update user permissions") {
		return
	}
}
