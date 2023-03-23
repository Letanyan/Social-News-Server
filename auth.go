package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Timothylock/go-signin-with-apple/apple"
	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"github.com/golang-jwt/jwt/v4"

	"github.com/awa/go-iap/appstore"
	"github.com/awa/go-iap/playstore"
)

var AUTHUserSecrets map[int64][]string
var AUTHUserReset sync.Map
var googleKeys map[string]string
var appleTokens sync.Map

func init() {
	AUTHUserSecrets = IndexSetFromFile[[]string]("user_secrets.gob")
	AUTHUserReset = sync.Map{}
	googleKeys = map[string]string{}
	appleTokens = sync.Map{}
}

// -------------------------------------------------------------------------
// AUTH User Requests
// -------------------------------------------------------------------------
func generateRandomString(length int) string {
	tokens := "1234567890qwertyuiopasdfghjklzxcvbnmQWERTYUIOPASDFGHJKLZXCVBNM"
	result := ""
	rand.Seed(time.Now().Unix())
	for i := 0; i < int(length); i++ {
		r := rand.Int31n(int32(len(tokens)))
		result += string(tokens[r])
	}
	return result
}

func AUTHRegister(db *sql.DB, user int64, deviceId string, ip string) string {
	result := generateRandomString(64)
	updateSecret := fmt.Sprintf(`
	INSERT INTO UserAuth (userId, deviceId, secret, key)
	VALUES (%d, '%s', '%s', '%s')
	ON CONFLICT (userId, deviceId) 
	DO UPDATE SET secret='%s', lastAction=(now() at time zone 'utc');
	`, user, DBAlphaNumeric(deviceId), DBAlphaNumeric(result), DBAlphaNumeric(ip), DBAlphaNumeric(result))
	_, e := db.Exec(updateSecret)
	if DidFail(e, "upsert secret ", result, " for user ", user, " on device ", deviceId) {
		return ""
	}
	return result
}

func AUTHRemoveOldSecrets(db *sql.DB, monthsAgo int) {
	query := fmt.Sprintf(`
	DELETE FROM UserAuth
	WHERE lastAction < ((now() at time zone 'utc') - interval '%d month')
	`, monthsAgo)
	_, e := db.Exec(query)
	DidFail(e, "delete old user auth secrets", query)
}

func AUTHGetSecret(db sql.DB, user int64, deviceId string, ip string) string {
	getSecret := fmt.Sprintf(`
	SELECT secret
	FROM UserAuth
	WHERE userId=%d AND deviceId='%s' AND key='%s'
	`, user, DBAlphaNumeric(deviceId), DBAlphaNumeric(ip))
	rows, e := db.Query(getSecret)
	if DidFail(e, "get secret for user ", user, " deviceId ", deviceId) {
		return ""
	}
	defer rows.Close()
	var secret string
	for rows.Next() {
		e := rows.Scan(&secret)
		if DidFail(e, "scan user auth secret") {
			continue
		}
		return secret
	}
	return ""
}

func AUTHUpdateLastAction(db *sql.DB, user int64, deviceId string, ip string) {
	update := fmt.Sprintf(`
	UPDATE UserAuth
	SET lastAction = (now() at time zone 'utc')
	WHERE userId=%d AND deviceId='%s' AND key='%s'
	`, user, DBAlphaNumeric(deviceId), DBAlphaNumeric(ip))
	_, e := db.Exec(update)
	DidFail(e, "update user ", user, " auth last action on device ", deviceId)
}

func AUTHMatchSecret(db *sql.DB, user int64, deviceId string, secret string, ip string) bool {
	findMatches := fmt.Sprintf(`
	SELECT secret
	FROM UserAuth
	WHERE userId=%d AND deviceId='%s' AND secret='%s' AND key='%s'
	`, user, DBAlphaNumeric(deviceId), DBAlphaNumeric(secret), DBAlphaNumeric(ip))

	rows, e := db.Query(findMatches)
	if DidFail(e, "get secret for user ", user, " deviceId ", deviceId) {
		return false
	}
	defer rows.Close()
	var s string
	for rows.Next() {
		e := rows.Scan(&s)
		if DidFail(e, "scan user auth secret") {
			continue
		}
		if secret == s {
			AUTHUpdateLastAction(db, user, deviceId, ip)
			return true
		}
	}
	// Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36
	// A1lQ60vQ
	return false
}

func AUTHDeregister(db *sql.DB, user int64, deviceId string, ip string) bool {
	removeUserAuth := fmt.Sprintf(`
	DELETE FROM UserAuth
	WHERE userId=%d AND deviceId='%s' AND key='%s'
	`, user, DBAlphaNumeric(deviceId), DBAlphaNumeric(ip))
	_, e := db.Exec(removeUserAuth)
	return !DidFail(e, "remove secret for user ", user, " and device ", deviceId)
}

// -------------------------------------------------------------------------
// AUTH IAP
// -------------------------------------------------------------------------

func AUTHAppleIAP_URL(url string, receipt string) int {
	reqBody, _ := json.Marshal(gin.H{
		"reciept-data":             receipt,
		"password":                 "30e299815ad94aa9988d07631f51e773",
		"exclude-old-transactions": false,
	})

	res, e := http.Post(url, "application/json; charset=UTF-8", bytes.NewBuffer(reqBody))
	if DidFail(e, "get apple iap receipt verify") {
		return -1
	}
	defer res.Body.Close()
	body, e := io.ReadAll(res.Body)
	if DidFail(e, "read apple iap body") {
		return -1
	}
	type Output struct {
		status int
	}
	var status Output
	e = json.Unmarshal(body, &status)
	if DidFail(e, "parse apple iap response") {
		return -1
	}

	return status.status
}

func AUTHAppleIAP(receipt string) bool {
	client := appstore.New()
	req := appstore.IAPRequest{
		ReceiptData:            receipt,
		Password:               "30e299815ad94aa9988d07631f51e773",
		ExcludeOldTransactions: false,
	}
	result := &appstore.IAPResponse{}
	ctx := context.Background()
	e := client.Verify(ctx, req, result)
	if DidFail(e, "verify app store purchase") {
		return false
	}
	return result.Status == 0
}

func AUTHGoogleIAP(receipt string, productId string) bool {
	jsonKey, e := os.ReadFile("gapi.json")
	if DidFail(e, "read google api json") {
		return false
	}

	client, e := playstore.New(jsonKey)
	if DidFail(e, "create playstore") {
		return false
	}

	ctx := context.Background()
	result, e := client.VerifyProduct(ctx, "com.letanyan.newsource", productId, receipt)
	if DidFail(e, "verify play store product") {
		return false
	}
	return result.PurchaseState == 0
}

func mapProductIdToCredit(id string) int64 {
	switch id {
	case "credit5":
		return 5
	case "credit15":
		return 15
	case "credit30":
		return 30
	case "credit50":
		return 50
	case "credit100":
		return 100
	}
	return -1
}

// -------------------------------------------------------------------------
// AUTH Sign In
// -------------------------------------------------------------------------

type GoogleClaims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	FirstName     string `json:"given_name"`
	LastName      string `json:"family_name"`
	jwt.RegisteredClaims
}

func getGooglePublicKey(keyId string) (string, error) {
	if key, found := googleKeys[keyId]; found {
		return key, nil
	}

	resp, e := http.Get("https://www.googleapis.com/oauth2/v1/certs")
	if DidFail(e, "get google public key") {
		return "", e
	}
	dat, e := io.ReadAll(resp.Body)
	if DidFail(e, "read response body") {
		return "", e
	}

	response := map[string]string{}
	e = json.Unmarshal(dat, &response)
	if DidFail(e, "unmarshal json body") {
		return "", e
	}
	key, ok := response[keyId]
	if !ok {
		return "", errors.New("key not found")
	}

	for k, v := range response {
		googleKeys[k] = v
	}

	return key, nil
}

func ValidateGoogleJWT(tokenString string) (GoogleClaims, error) {
	claimsStruct := GoogleClaims{}

	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	token, e := parser.ParseWithClaims(
		tokenString,
		&claimsStruct,
		func(token *jwt.Token) (interface{}, error) {
			pem, e := getGooglePublicKey(fmt.Sprintf("%s", token.Header["kid"]))
			if DidFail(e, "get google public key") {
				return nil, e
			}
			key, e := jwt.ParseRSAPublicKeyFromPEM([]byte(pem))
			if DidFail(e, "parse ras public key from PEM") {
				return nil, e
			}
			return key, nil
		},
	)
	if DidFail(e, "parse claims") {
		return GoogleClaims{}, e
	}

	claims, ok := token.Claims.(*GoogleClaims)
	if !ok {
		return GoogleClaims{}, errors.New("invalid google jwt")
	}

	if claims.Issuer != "accounts.google.com" && claims.Issuer != "https://accounts.google.com" {
		return GoogleClaims{}, errors.New("iss is invalid")
	}

	hasAudience := false
	for _, s := range claims.Audience {
		if s == "543660397854-h1sodc8t5htp81k9lihs00kojdnrupfe.apps.googleusercontent.com" {
			hasAudience = true
		}
		if s == "543660397854-qobpehta8eajd750gr4f4n0cv1n23m3s.apps.googleusercontent.com" {
			hasAudience = true
		}
	}
	if !hasAudience {
		return GoogleClaims{}, errors.New("aud is invalid")
	}

	if claims.ExpiresAt.Unix() < time.Now().UTC().Unix() {
		return GoogleClaims{}, errors.New("JWT is expired")
	}

	return *claims, nil
}

func ValidateAppleJWT(code string) GoogleClaims {
	token, loaded := appleTokens.LoadAndDelete(code)
	if !loaded {
		serverToken, e := ValidateAppleJWTServer(code)
		token = serverToken
		if DidFail(e, "validate apple on server") {
			return GoogleClaims{}
		}
	}
	claimsStruct := GoogleClaims{}
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	parser.ParseUnverified(token.(string), &claimsStruct)

	return claimsStruct
}

func ValidateAppleJWTServer(code string) (string, error) {
	clientId := "com.letanyan.newsource"
	privateKey := `-----BEGIN PRIVATE KEY-----
MIGTAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBHkwdwIBAQQgX8e/+ExMOMTbLzav
lg8rFYOhBfeGrcAIKL+7Q4FjjSGgCgYIKoZIzj0DAQehRANCAATtI0L8/MPp2b4T
J6/1jA9dnkP0SodRODScM2opvHJgKYhevNPi/Blu5pd3ble2zGBctKdDbHpW6Xf3
jAhjLaPD
-----END PRIVATE KEY-----`
	teamId := "86QZ48F54E"
	keyId := "K3NQ5VC2LH"

	secret, e := apple.GenerateClientSecret(privateKey, teamId, clientId, keyId)

	if DidFail(e, "generate client secret") {
		return "", e
	}

	client := apple.New()

	req := apple.AppValidationTokenRequest{
		ClientID:     clientId,
		ClientSecret: secret,
		Code:         code,
	}

	var resp apple.ValidationResponse

	// Do the verification
	e = client.VerifyAppToken(context.Background(), req, &resp)
	if DidFail(e, "app token verify") {
		return "", e
	}

	if resp.Error != "" {
		fmt.Printf("apple returned an error: %s - %s\n", resp.Error, resp.ErrorDescription)
		if e != nil {
			return "", e
		}
	}

	return resp.IDToken, nil

	// Get the unique user ID
	// userId, e := apple.GetUniqueID(resp.IDToken)
	// if DidFail(e, "get unique id") {
	// 	return "", e
	// }

	// // Get the email
	// claim, err := apple.GetClaims(resp.IDToken)
	// if DidFail(e, "get claims") {
	// 	return "", err
	// }

	// email := (*claim)["email"].(string)

	// return &AuthenticatedAppleUser{
	// 	AppleUserId: userId,
	// 	Email:       strings.TrimSpace(strings.ToLower(email)),
	// }, nil
}
