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
	UpdatedAt     time.Time
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
	return "id, name, email, password, registerDate, updatedAt, upvotes, downvotes, credits, validationKey"
}

func SQLFieldsForUserProfile() string {
	return "id, name, registerDate, upvotes, downvotes"
}

func ScanUser(row *sql.Row) (User, error) {
	u := User{}
	e := row.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.RegisterDate, &u.UpdatedAt, &u.Upvotes, &u.Downvotes, &u.Credits, &u.ValidationKey)
	return u, e
}

func ScanUserProfile(row *sql.Row) (UserProfile, error) {
	u := UserProfile{}
	e := row.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes)
	return u, e
}

func ScanUserProfiles(rows *sql.Rows, includeScore bool) []User {
	result := []User{}
	var e error
	for rows.Next() {
		user := User{}
		var score float64
		var cred float64
		if includeScore {
			e = rows.Scan(&user.ID, &user.Name, &user.Upvotes, &user.Downvotes)
		} else {
			e = rows.Scan(&user.ID, &user.Name, &user.Upvotes, &user.Downvotes, &cred, &score)
		}
		// if excludeImp {
		// 	user.Email = ""
		// 	user.Password = ""
		// 	user.RegisterDate = time.Time{}
		// 	user.UpdatedAt = time.Time{}
		// 	user.ValidationKey = 0
		// }
		if DidFail(e, "get user from email/id") {
			continue
		}
		result = append(result, user)
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
	return fmt.Sprintf("%x", h.Sum(nil))
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

	createUserContentTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS User%dCont (
		postId BIGINT NOT NULL,
		commentId BIGINT NOT NULL,

		PRIMARY KEY (postId, commentId)
	);`, user.ID)
	_, e = db.Exec(createUserContentTable)
	if DidFail(e, "create user content table for user ", user.ID) {
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
	);`, user.ID)
	_, e = db.Exec(createUserPrefTable)
	if DidFail(e, "create user preference table for user ", user.ID) {
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
	getUser := fmt.Sprintf(`SELECT %s FROM users WHERE `, SQLFieldsForUser())
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

func DBGetUsers(db *sql.DB, upvotes int64, downvotes int64, sortOrder SortOrder, limit int64, offset int64) []User {
	getUsers := fmt.Sprintf(`SELECT %s, RATIO(upvotes, downvotes) AS cred, upvotes * RATIO(upvotes, downvotes) AS score  
	FROM users`, SQLFieldsForUserProfile())

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

	getUsers += SQLSortOrder(sortOrder, "")
	getUsers += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(getUsers)
	if DidFail(e, "get users") {
		return []User{}
	}

	result := ScanUserProfiles(rows, true)

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

func DBVoteForUser(db *sql.DB, userId int64, targetId int64, upvoteAmount int64) (UserProfile, UserPref) {
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
	credits = credits + 0.75 * %d,
	updatedAt = '%s'
	WHERE id = $1
	RETURNING %s
	`, updatedField, updatedField, nowTime, upvoteAmount, otherField, otherField, nowTime, upvoteAmount, nowTime, SQLFieldsForUserProfile())
	row := db.QueryRow(updateUser, targetId)
	user, e := ScanUserProfile(row)
	if DidFail(e, "update user score", targetId) {
		return UserProfile{}, UserPref{}
	}

	vote := fmt.Sprintf(`INSERT INTO User%dPref (kind, pid, sid)
	VALUES (1, %d, -1) ON CONFLICT (kind, pid, sid) DO NOTHING;
	UPDATE User%dPref 
	SET %s = cooldown(%s, updatedAt, '%s', 31536000) + %d,
	%s = cooldown(%s, updatedAt, '%s', 31536000)
	WHERE kind=1 AND pid = %d
	RETURNING kind, pid, sid, upvotes, downvotes
	`, userId, targetId, userId, updatedField, updatedField, nowTime, upvoteAmount, otherField, otherField, nowTime, targetId)
	row = db.QueryRow(vote)
	pref, e := ScanUserPrefRow(row)
	if DidFail(e, "vote for user") {
		return UserProfile{}, UserPref{}
	}

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
