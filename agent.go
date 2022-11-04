package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
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
	for _, a := range agents {
		if a.LastUpdate.Before(utc().Add(before)) {
			result = append(result, NAUpdateNewsAgent(a))
		}
	}
	return result
}

func NAUpdateNewsAgent(agent NewsAgent) WebsiteScrapings {
	if updatingAgents {
		return WebsiteScrapings{}
	}
	updatingAgents = true
	if agent.ID == 0 {
		updatingAgents = false
		return WebsiteScrapings{}
	}
	scraping := NAScrapeWebsite(agent.Origin)

	if scraping.Type == "article" {
		NACreatePost(agent.ID, agent.Origin, scraping)
	}
	baseURL, e := nurl.Parse(agent.Origin)
	if DidFail(e, "invalid origin url") {
		updatingAgents = false
		return WebsiteScrapings{}
	}
	hashFile := fmt.Sprintf("./agents/%d.gob", agent.ID)
	visited := HashSetFromFile(hashFile)
	defer HashSetWriteToFile(visited, hashFile)
	allUrls := NATraverseSubDomains(agent.Origin, agent.Subs)
	allUrls = append(allUrls, scraping.URLs...)
	for _, urlString := range allUrls {
		url, e := nurl.Parse(urlString)
		canURLString := CanonicalURL(url.String())
		if e != nil || !IsSameHost(url, baseURL) {
			continue
		}
		canURLString = strings.TrimPrefix(canURLString, baseURL.Hostname())
		if !visited[canURLString] {
			visited[canURLString] = true
			subScraping := NAScrapeWebsite(urlString)
			if subScraping.Type == "article" {
				NACreatePost(agent.ID, urlString, subScraping)
			}
		}
	}
	updatingAgents = false
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

	req, _ := http.NewRequest(http.MethodGet, url, nil)
	// req.Header.Set("User-Agent", "Journo/0.1 (Windows NT 10; Win64; x64)")
	res, _ := client.Do(req)
	if res.StatusCode != 200 {
		return WebsiteScrapings{}
	}

	bodyNode, err := html.Parse(res.Body)
	if err != nil {
		println(err.Error())
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
	var titleNode *html.Node
	var metaTitleNode *html.Node
	links := []string{}
	tags := []string{}
	image := ""
	description := ""
	authors := []string{}
	contentType := ""
	modTime := ""
	pubTime := ""

	getHTMLNodes(node, false, func(n *html.Node) bool {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "title":
				titleNode = n
			case "meta":
				properties := getValForAttr(n, "property")
				names := getValForAttr(n, "name")
				if names["og:title"] || properties["og:title"] {
					metaTitleNode = n
				} else if names["article:tag"] || properties["article:tag"] {
					tags = append(tags, tagFormat(getFirstValForAttr(n, "content")))
				} else if names["og:image"] || properties["og:image"] {
					image = getFirstValForAttr(n, "content")
				} else if names["og:description"] || properties["og:description"] {
					description = getFirstValForAttr(n, "content")
				} else if names["article:author"] || properties["article:author"] {
					authors = append(authors, getFirstValForAttr(n, "content"))
				} else if names["article:section"] || properties["article:section"] {
					tags = append(tags, tagFormat(getFirstValForAttr(n, "content")))
				} else if names["og:type"] || properties["og:type"] {
					contentType = getFirstValForAttr(n, "content")
				} else if names["article:modified_time"] || properties["article:modified_time"] {
					modTime = getFirstValForAttr(n, "content")
				} else if names["article:published_time"] || properties["article:published_time"] {
					pubTime = getFirstValForAttr(n, "content")
				}
			case "a":
				links = append(links, getFirstValForAttr(n, "href"))
			}
		}
		return false
	})

	titleText := ""
	metaText := ""

	if titleNode != nil {
		titleText = titleNode.FirstChild.Data
	}
	if metaTitleNode != nil {
		for k := range getValForAttr(metaTitleNode, "content") {
			metaText = k
		}
	}

	filterTitleText := replaceUnicode(titleText, isNotAlphanumeric)
	filterMetaText := replaceUnicode(metaText, isNotAlphanumeric)

	if filterMetaText != "" && filterMetaText != filterTitleText && strings.HasPrefix(filterTitleText, filterMetaText) {
		titleText = metaText
	}

	title := strings.TrimSpace(titleText)

	date := utc()
	if len(modTime) > 0 {
		date = parseUnknownTime(modTime)
	} else if len(pubTime) > 0 {
		date = parseUnknownTime(pubTime)
	}

	return WebsiteScrapings{title, description, links, tags, authors, image, contentType, date}
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

	DBCreatePost(mainDB, userId, body, scrape.Date, scrape.Tags, []string{})
}

func NARegisterUpdates() {
	time.AfterFunc(0, func() { NAUpdateAllNewsAgent(time.Minute * 30) })
	time.AfterFunc(time.Hour, func() { NARegisterUpdates() })
}
