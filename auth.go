package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"golang.org/x/oauth2/google"
)

var AUTHUserSecrets map[int64][]string

func init() {
	AUTHUserSecrets = AUTHLoadFromFile("user_secrets.gob")
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
	if data, hasKey := AUTHUserSecrets[user]; hasKey {
		data = append(data, result)
		AUTHUserSecrets[user] = data
	} else {
		AUTHUserSecrets[user] = []string{result}
	}
	AUTHWriteToFile(AUTHUserSecrets, "user_secrets.gob")
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
		delete(AUTHUserSecrets, user)
		AUTHWriteToFile(AUTHUserSecrets, "user_secrets.gob")
	} else if len(secrets) > 1 {
		j := -1
		for i, s := range secrets {
			if s == secret {
				j = i
				break
			}
		}
		if j >= 0 {
			secrets[j] = secrets[len(secrets)-1]
			AUTHUserSecrets[user] = secrets[:len(secrets)-1]
			AUTHWriteToFile(AUTHUserSecrets, "user_secrets.gob")
		}
	}
}

// -------------------------------------------------------------------------
// AUTH IAP
// -------------------------------------------------------------------------

func AUTHIAPAppleURL(url string, receipt string) int {
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

func AUTHIAPApple(receipt string) bool {
	productionURL := "https://buy.itunes.apple.com/verifyReceipt"
	sandboxURL := "https://sandbox.itunes.apple.com/verifyReceipt"

	status := AUTHIAPAppleURL(productionURL, receipt)
	if status == 0 {
		return true
	} else if status == 21007 {
		return AUTHIAPAppleURL(sandboxURL, receipt) == 0
	} else {
		return false
	}
}

type GoogleIAP struct {
	Kind               string `json:"kind"`
	PurchaseTimeMillis string `json:"purchaseTimeMillis"`
	PurchaseState      string `json:"purchaseState"`
	ConsumptionState   bool   `json:"consumptionState"`
	DeveloperPayload   string `json:"developerPayload"`
}

func AUTHAPIGoogle(receipt string) bool {
	data, e := ioutil.ReadFile("gapi.json")
	if DidFail(e, "read jwt") {
		return false
	}
	conf, e := google.JWTConfigFromJSON(data, "https://www.googleapis.com/auth/purchases.product")
	if DidFail(e, "get JWT config json") {
		return false
	}
	client := conf.Client(context.Background())
	packageName := "com.letanyan.newsource"
	productId := ""
	res, e := client.Get(fmt.Sprintf("https://androidpublisher.googleapis.com/androidpublisher/v3/applications/%s/purchases/products/%s/tokens/%s", packageName, productId, receipt))
	if DidFail(e, "get product") {
		return false
	}
	body, e := ioutil.ReadAll(res.Body)
	if DidFail(e, "read google iap body") {
		return false
	}
	type Output struct {
		resource GoogleIAP
	}
	var status Output
	e = json.Unmarshal(body, &status)
	if DidFail(e, "parse google iap response") {
		return false
	}
	return status.resource.PurchaseState == "0"
}
