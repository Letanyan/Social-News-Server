package main

import (
	"math/rand"
	"time"
)

var AUTHUserSecrets map[int64]string

func AUTHRegister(user int64) string {
	tokens := "1234567890qwertyuiopasdfghjklzxcvbnmQWERTYUIOPASDFGHJKLZXCVBNM"
	result := ""
	rand.Seed(time.Now().Unix())
	for i := 0; i < 12; i++ {
		r := rand.Int31n(int32(len(tokens)))
		result += string(tokens[r])
	}
	AUTHUserSecrets[user] = result
	return result
}

func AUTHGetSecret(user int64) string {
	return AUTHUserSecrets[user]
}

func AUTHMatchSecret(user int64, secret string) bool {
	return AUTHUserSecrets[user] == secret
}

func AUTHDeregister(user int64) {
	delete(AUTHUserSecrets, user)
}

func init() {
	AUTHUserSecrets = map[int64]string{}
}
