package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/Timothylock/go-signin-with-apple/apple"
	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"github.com/golang-jwt/jwt/v4"

	"github.com/awa/go-iap/appstore"
	"github.com/awa/go-iap/playstore"
)

var userSecretMutex sync.Mutex
var AUTHUserSecrets map[int64][]string
var googleKeys map[string]string
var appleTokens sync.Map

type InAppPurchase struct {
	Platform  string
	ProductId string
	Data      string
}

var iapMutex sync.Mutex
var inAppPurchases map[int64][]InAppPurchase

func init() {
	userSecretMutex = sync.Mutex{}
	AUTHUserSecrets = IndexSetFromFile[[]string]("user_secrets.gob")
	googleKeys = map[string]string{}
	appleTokens = sync.Map{}
	iapMutex = sync.Mutex{}
	inAppPurchases = IndexSetFromFile[[]InAppPurchase]("user_iap.gob")
}

// -------------------------------------------------------------------------
// AUTH User Requests
// -------------------------------------------------------------------------

func AUTHRegister(user int64) string {
	tokens := "1234567890qwertyuiopasdfghjklzxcvbnmQWERTYUIOPASDFGHJKLZXCVBNM"
	result := ""
	rand.Seed(time.Now().Unix())
	for i := 0; i < 12; i++ {
		r := rand.Int31n(int32(len(tokens)))
		result += string(tokens[r])
	}
	userSecretMutex.Lock()
	if data, hasKey := AUTHUserSecrets[user]; hasKey {
		if len(data) > 5 {
			data = data[1:]
		}
		data = append(data, result)
		AUTHUserSecrets[user] = data
	} else {
		AUTHUserSecrets[user] = []string{result}
	}
	IndexSetWriteToFile(AUTHUserSecrets, "user_secrets.gob")
	userSecretMutex.Unlock()
	return result
}

func AUTHGetSecret(user int64) []string {
	return AUTHUserSecrets[user]
}

func AUTHMatchSecret(user int64, secret string) bool {
	secrets := AUTHUserSecrets[user]
	for _, s := range secrets {
		if s == secret {
			return true
		}
	}
	return false
}

func AUTHDeregister(user int64, secret string) {
	secrets := AUTHGetSecret(user)
	if len(secrets) == 1 {
		userSecretMutex.Lock()
		delete(AUTHUserSecrets, user)
		IndexSetWriteToFile(AUTHUserSecrets, "user_secrets.gob")
		userSecretMutex.Unlock()
	} else if len(secrets) > 1 {
		j := -1
		for i, s := range secrets {
			if s == secret {
				j = i
				break
			}
		}
		if j >= 0 {
			userSecretMutex.Lock()
			secrets[j] = secrets[len(secrets)-1]
			AUTHUserSecrets[user] = secrets[:len(secrets)-1]
			IndexSetWriteToFile(AUTHUserSecrets, "user_secrets.gob")
			userSecretMutex.Unlock()
		}
	}
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
	body, e := ioutil.ReadAll(res.Body)
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
	jsonKey, e := ioutil.ReadFile("gapi.json")
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

func AUTHAlreadyPurchased(userId int64, data string, productId string, platform string) bool {
	purchases := inAppPurchases[userId]
	for _, purchase := range purchases {
		if purchase.Data == data && purchase.ProductId == productId &&
			purchase.Platform == platform {
			return true
		}
	}
	purchase := InAppPurchase{Platform: platform, ProductId: productId, Data: data}
	purchases = append(purchases, purchase)
	iapMutex.Lock()
	inAppPurchases[userId] = purchases
	IndexSetWriteToFile(inAppPurchases, "user_iap.gob")
	iapMutex.Unlock()
	return false
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
	dat, e := ioutil.ReadAll(resp.Body)
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
