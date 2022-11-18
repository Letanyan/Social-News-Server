package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	nurl "net/url"

	"golang.org/x/net/html"
)

type NewsAgent struct {
	ID         int64
	Name       string
	Origin     string
	LastUpdate time.Time
	Subs       []string
}

type WebsiteScrapings struct {
	Title       string
	Description string
	URLs        []string
	Tags        []string
	Authors     []string
	Image       string
	Type        string
	Locale      string
	Language    string
	Date        time.Time
}

func NACreateNewsAgent(id int64, name string, origin string) (NewsAgent, error) {
	e := NAAgentExists(name, origin)
	if e != nil {
		return NewsAgent{}, e
	}
	agent := NewsAgent{id, name, origin, utc(), []string{}}
	agents = append(agents, agent)
	NAWriteAllNewsAgents(agents)
	visited := map[string]bool{}
	HashSetWriteToFile(visited, fmt.Sprintf("./agents/%d.gob", id))
	return agent, nil
}

func NAEditNewsAgent(id int64, name string, origin string) NewsAgent {
	var found = -1
	for i, a := range agents {
		if a.ID == id {
			found = i
			break
		}
	}
	if found == -1 {
		return NewsAgent{}
	}
	if name != agents[found].Name {
		DBUpdateUser(mainDB, id, name)
	}
	agents[found].Name = name
	agents[found].Origin = origin
	NAWriteAllNewsAgents(agents)

	return agents[found]
}

func NAAddSub(id int64, dir string) NewsAgent {
	var found = -1
	for i, a := range agents {
		if a.ID == id {
			found = i
			break
		}
	}
	if found == -1 {
		return NewsAgent{}
	}
	agents[found].Subs = append(agents[found].Subs, dir)
	NAWriteAllNewsAgents(agents)

	return agents[found]
}

func NARemoveSub(id int64, dir string) NewsAgent {
	var found = -1
	for i, a := range agents {
		if a.ID == id {
			found = i
			break
		}
	}
	if found == -1 {
		return NewsAgent{}
	}

	agent := agents[found]
	remove := -1
	for i, s := range agent.Subs {
		if s == dir {
			remove = i
			break
		}
	}
	if remove == -1 {
		return agent
	}

	agent.Subs[remove] = agent.Subs[len(agent.Subs)-1]
	agents[found].Subs = agent.Subs[:len(agent.Subs)-1]
	NAWriteAllNewsAgents(agents)

	return agents[found]
}

func NAAgentExists(name string, origin string) error {
	for _, agent := range agents {
		if agent.Name == name {
			return errors.New("agent with name already exists")
		}
		if agent.Origin == origin {
			return errors.New("agent with origin already exists")
		}
	}
	return nil
}

func NADeleteNewsAgent(id int64) error {
	idx := -1
	for i, a := range agents {
		if a.ID == id {
			idx = i
		}
	}

	if idx > 0 {
		return errors.New("agent does not exist with id " + fmt.Sprint(idx))
	}

	agents[idx] = agents[len(agents)-1]
	agents = agents[:len(agents)-1]

	return nil
}

func NAReadAllNewsAgents() []NewsAgent {
	content, e := ioutil.ReadFile("./agents/agents.json")
	if DidFail(e, "read agents.json file") {
		return []NewsAgent{}
	}

	var result []NewsAgent
	e = json.Unmarshal(content, &result)
	if DidFail(e, "unmarshal news agents") {
		return []NewsAgent{}
	}

	createdNew := false
	for i := range result {
		agent := result[i]
		oldUser := DBGetUser(mainDB, agent.ID, "")
		if oldUser.ID == 0 || oldUser.Email != "" {
			createdNew = true
			user := DBCreateUser(mainDB, agent.Name, "", "")
			DBValidateUser(mainDB, user.ID, user.ValidationKey)
			result[i].ID = user.ID
			visited := map[string]bool{}
			HashSetWriteToFile(visited, fmt.Sprintf("./agents/%d.gob", user.ID))
		}
	}
	if createdNew && len(result) > 0 {
		NAWriteAllNewsAgents(result)
	}

	return result
}

func NAWriteAllNewsAgents(agents []NewsAgent) {
	file, e := json.Marshal(agents)
	if DidFail(e, "marshal agents") {
		return
	}

	e = ioutil.WriteFile("./agents/agents.json", file, 0644)
	if DidFail(e, "write agents to file") {
		return
	}
}

func NAUpdateNewsAgentWithID(id int64) WebsiteScrapings {
	var agent NewsAgent
	for _, a := range agents {
		if a.ID == id {
			agent = a
			break
		}
	}
	return NAUpdateNewsAgent(agent)
}

func NAUpdateAllNewsAgent(before time.Duration) []WebsiteScrapings {
	result := []WebsiteScrapings{}
	wg := sync.WaitGroup{}
	m := sync.Mutex{}
	for i := range agents {
		a := agents[i]
		if a.LastUpdate.Before(utc().Add(before)) {
			wg.Add(1)
			agents[i].LastUpdate = utc()
			go func() {
				scrape := NAUpdateNewsAgent(a)
				m.Lock()
				result = append(result, scrape)
				defer m.Unlock()
				wg.Done()
			}()
		}
	}
	wg.Wait()
	NAWriteAllNewsAgents(agents)
	return result
}

func NAUpdateNewsAgent(agent NewsAgent) WebsiteScrapings {
	if agent.ID == 0 {
		return WebsiteScrapings{}
	}
	scraping := NAScrapeWebsite(agent.Origin)

	if scraping.Type == "article" {
		NACreatePost(agent.ID, agent.Origin, scraping)
	}
	baseURL, e := nurl.Parse(agent.Origin)
	if DidFail(e, "invalid origin url") {
		return WebsiteScrapings{}
	}
	hashFile := fmt.Sprintf("./agents/%d.gob", agent.ID)
	if _, e = os.Stat(hashFile); errors.Is(e, os.ErrNotExist) {
		visited := map[string]bool{}
		HashSetWriteToFile(visited, fmt.Sprintf("./agents/%d.gob", agent.ID))
	}

	createPosts := func(allUrls []string, origin string) {
		unlock := agentsMutex.Lock(origin)
		defer unlock()
		visited := HashSetFromFile(hashFile)
		defer HashSetWriteToFile(visited, hashFile)
		for _, urlString := range allUrls {
			url, e := nurl.Parse(strings.TrimSpace(urlString))
			if DidFail(e, "parse url", urlString) {
				continue
			}
			if !IsSameHost(url, baseURL) {
				continue
			}
			canURLString := CanonicalURL(url.String())
			canURLString = strings.TrimPrefix(canURLString, baseURL.Hostname())
			if !visited[canURLString] {
				visited[canURLString] = true
				subScraping := NAScrapeWebsite(urlString)
				if subScraping.Type == "article" {
					NACreatePost(agent.ID, urlString, subScraping)
				}
			}
		}
	}

	go createPosts(scraping.URLs, agent.Origin)
	for i := range agent.Subs {
		sub := agent.Subs[i]
		fullUrl := agent.Origin + sub
		subScrape := NAScrapeWebsite(fullUrl)
		var offset = time.Second * time.Duration(i+1) * 10
		anon := func() {
			createPosts(subScrape.URLs, agent.Origin)
		}
		time.AfterFunc(offset, anon)
	}

	return scraping
}

func CanonicalURL(a string) string {
	x := strings.TrimPrefix(a, "https://")
	x = strings.TrimPrefix(x, "http://")
	x = strings.TrimPrefix(x, "www.")
	x = strings.TrimSuffix(x, "/")
	return x
}

func IsSameHost(a *nurl.URL, b *nurl.URL) bool {
	x := CanonicalURL(a.Hostname())
	y := CanonicalURL(b.Hostname())
	return x == y
}

func NATraverseSubDomains(origin string, subs []string) []string {
	result := []string{}
	for _, sub := range subs {
		scraping := NAScrapeWebsite(origin + sub)
		result = append(result, scraping.URLs...)
	}
	return result
}

func NAScrapeWebsite(url string) WebsiteScrapings {
	client := &http.Client{}

	baseUrl, e := nurl.Parse(url)
	if DidFail(e, "parse url for scraping") {
		return WebsiteScrapings{}
	}

	req, e := http.NewRequest(http.MethodGet, url, nil)
	if DidFail(e, "get url ", url) {
		return WebsiteScrapings{}
	}
	// req.Header.Set("User-Agent", "Journo/0.1 (Windows NT 10; Win64; x64)")
	res, e := client.Do(req)
	if DidFail(e, "make request ", url) {
		return WebsiteScrapings{}
	}

	if res.StatusCode != 200 {
		return WebsiteScrapings{}
	}

	bodyNode, e := html.Parse(res.Body)
	if DidFail(e, "parse html") {
		return WebsiteScrapings{}
	}

	result := NAReadData(bodyNode)

	for i, link := range result.URLs {
		nlink, e := nurl.Parse(link)
		if e != nil {
			continue
		}
		joined := baseUrl.ResolveReference(nlink)
		result.URLs[i] = joined.String()
	}

	return result
}

func NAReadData(node *html.Node) WebsiteScrapings {
	if node == nil {
		return WebsiteScrapings{}
	}
	var metaTitleNode *html.Node
	links := []string{}
	tags := []string{}
	image := ""
	description := ""
	authors := []string{}
	contentType := ""
	modTime := ""
	pubTime := ""
	locale := ""
	language := ""

	getHTMLNodes(node, false, func(n *html.Node) bool {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "meta":
				properties := getValForAttr(n, "property")
				names := getValForAttr(n, "name")
				if names["og:title"] || properties["og:title"] {
					metaTitleNode = n
				} else if names["article:tag"] || properties["article:tag"] {
					tags = append(tags, tagFormat(getFirstValForAttr(n, "content"))...)
				} else if names["og:image"] || properties["og:image"] {
					image = getFirstValForAttr(n, "content")
				} else if names["og:description"] || properties["og:description"] {
					description = getFirstValForAttr(n, "content")
				} else if names["article:author"] || properties["article:author"] {
					authors = append(authors, getFirstValForAttr(n, "content"))
				} else if names["article:section"] || properties["article:section"] {
					tags = append(tags, tagFormat(getFirstValForAttr(n, "content"))...)
				} else if names["og:type"] || properties["og:type"] {
					contentType = getFirstValForAttr(n, "content")
				} else if names["article:modified_time"] || properties["article:modified_time"] {
					modTime = getFirstValForAttr(n, "content")
				} else if names["article:published_time"] || properties["article:published_time"] {
					pubTime = getFirstValForAttr(n, "content")
				} else if names["og:locale"] || properties["og:locale"] {
					locale = getFirstValForAttr(n, "content")
				} else if names["og:language"] || properties["og:language"] {
					language = getFirstValForAttr(n, "content")
				} else if names["og:section"] || properties["og:section"] {
					tags = append(tags, tagFormat(getFirstValForAttr(n, "content"))...)
				}
			case "a":
				links = append(links, getFirstValForAttr(n, "href"))
			}
		}
		return false
	})

	metaText := ""
	if metaTitleNode != nil {
		for k := range getValForAttr(metaTitleNode, "content") {
			metaText = k
		}
	}
	title := strings.TrimSpace(metaText)

	date := utc()
	if len(modTime) > 0 {
		date = parseUnknownTime(modTime)
	} else if len(pubTime) > 0 {
		date = parseUnknownTime(pubTime)
	}

	return WebsiteScrapings{title, description, links, tags, authors, image, contentType, locale, language, date}
}

func NACreatePost(userId int64, url string, scrape WebsiteScrapings) {
	body := url + "\n!" + scrape.Title

	if len(scrape.Image) > 0 {
		body += "\n" + scrape.Image
	}
	if len(scrape.Description) > 0 {
		body += "\n" + scrape.Description
	}
	if len(scrape.Authors) > 0 {
		body += "\n\nBy: "
		for i, author := range scrape.Authors {
			body += author
			if i < len(scrape.Authors)-1 {
				body += ", "
			}
		}
	}
	if scrape.Date.Year() != utc().Year() {
		createPostsPartitionTable(mainDB, scrape.Date.Year())
	}

	tags := scrape.Tags
	comps := strings.Split(url, "/")
	common := map[string]bool{
		"sport": true, "boxing": true, "football": true,
		"news": true, "politics": true, "money": true,
		"tv": true, "cricket": true, "travel": true,
		"gaming": true, "world": true, "business": true,
		"technology": true, "science": true, "lifestyle": true,
		"film": true, "movies": true, "family": true, "sex": true,
		"dieting": true, "weird": true, "crime": true, "health": true,
		"tech": true, "ufc": true, "rugby": true, "f1": true,
		"racing": true, "golf": true, "tennis": true, "athletics": true,
		"darts": true, "snooker": true, "baseball": true,
		"basketball": true, "celebrity": true,
		"weird-news": true, "africa": true, "americas": true,
		"china": true, "asia": true, "asia-pacific": true,
		"europe": true, "india": true, "middle-east": true,
		"united-kingdom": true, "uk": true, "us": true,
		"energy": true, "environment": true, "finance": true,
	}
	for _, comp := range comps {
		t := strings.ToLower(comp)
		if _, found := common[t]; found {
			tags = append(tags, t)
		}
	}

	DBCreatePost(mainDB, userId, body, scrape.Date, tags, []string{})
}

func NARegisterUpdates() {
	time.AfterFunc(0, func() { NAUpdateAllNewsAgent(time.Minute * 30) })
	time.AfterFunc(time.Hour, func() { NARegisterUpdates() })
}
