package main

type Address struct {
	ContinentCode string `json:"continentCode"`
	CountryCode   string `json:"countryCode"`
	Region        string `json:"region"`
	City          string `json:"city"`
	Zip           string `json:"zip"`
}

func getAddress(ip string) []string {
	return []string{}

	// qry := fmt.Sprintf("http://ip-api.com/json/%s?fields=continentCode,countryCode,regionName,city,zip", ip)
	// res, e := http.Get(qry)
	// if DidFail(e, "get ip-api location") {
	// 	return []string{}
	// }
	// defer res.Body.Close()
	// body, e := ioutil.ReadAll(res.Body)
	// if DidFail(e, "read ip-api body") {
	// 	return []string{}
	// }
	// var addr Address
	// e = json.Unmarshal(body, &addr)
	// if DidFail(e, "parse ip-api json result to Address") {
	// 	return []string{}
	// }
	// return []string{addr.countryCode, addr.region}
}
