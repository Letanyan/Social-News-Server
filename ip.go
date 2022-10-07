package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Address struct {
	ContinentCode string
	CountryCode   string
	RegionName    string
	City          string
	Zip           string
}

func getAddress(ip string) []string {
	qry := fmt.Sprintf("http://ip-api.com/json/%s?fields=continentCode,countryCode,regionName,city,zip", ip)
	res, e := http.Get(qry)
	if DidFail(e, "get ip-api location") {
		return []string{}
	}
	defer res.Body.Close()
	body, e := ioutil.ReadAll(res.Body)
	if DidFail(e, "read ip-api body") {
		return []string{}
	}
	var addr Address
	e = json.Unmarshal(body, &addr)
	if DidFail(e, "parse ip-api json result to Address") {
		return []string{}
	}
	return []string{addr.ContinentCode, addr.CountryCode, addr.RegionName, addr.City, addr.Zip}
}
