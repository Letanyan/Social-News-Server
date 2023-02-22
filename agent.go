package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
	Locale     string
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

func NACreateNewsAgent(id int64, name string, origin string, locale string) (NewsAgent, error) {
	e := NAAgentExists(name, origin)
	if e != nil {
		return NewsAgent{}, e
	}
	agent := NewsAgent{id, name, locale, origin, utc(), []string{}}
	agents = append(agents, agent)
	NAWriteAllNewsAgents(agents)
	return agent, nil
}

func NAEditNewsAgent(db *sql.DB, id int64, name string, origin string) NewsAgent {
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
		DBUpdateUser(db, id, name)
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

func NAReadAllNewsAgents(db *sql.DB) []NewsAgent {
	content, e := os.ReadFile("./agents/agents.json")
	if DidFail(e, "read agents.json file") {
		return []NewsAgent{}
	}

	var result []NewsAgent
	e = json.Unmarshal(content, &result)
	if DidFail(e, "unmarshal news agents") {
		return []NewsAgent{}
	}

	updated := false
	agentNames := map[string]bool{}
	for i := range result {
		agent := result[i]
		oldUser := DBGetUserAgent(db, agent.Name)
		if _, found := agentNames[oldUser.Name]; found {
			fail.Println("duplicate agent name ", oldUser.Name)
			continue
		}
		agentNames[agent.Name] = true
		if oldUser.ID == 0 {
			updated = true
			user := DBCreateUser(db, agent.Name, "", "")
			DBValidateUser(db, user.ID, user.ValidationKey)
			result[i].ID = user.ID
		} else if oldUser.ID != agent.ID {
			updated = true
			result[i].ID = oldUser.ID
		}
	}
	if updated && len(result) > 0 {
		NAWriteAllNewsAgents(result)
	}

	return result
}

func NAReadAllAgentsOnboarding() map[string]int64 {
	result := map[string]int64{}
	for _, agent := range agents {
		result[agent.Name] = agent.ID
	}
	return result
}

func ConvertAllAgentsFromBoolToUnix() {
	content, e := os.ReadFile("./agents/agents.json")
	if DidFail(e, "read agents.json file") {
		return
	}

	var result []NewsAgent
	e = json.Unmarshal(content, &result)
	if DidFail(e, "unmarshal news agents") {
		return
	}

	for i := range result {
		agent := result[i]
		hashFile := fmt.Sprintf("./agents/%d.gob", agent.ID)
		visited := HashSetFromFile[bool](hashFile)
		converted := map[string]int64{}
		for k := range visited {
			converted[k] = time.Now().UTC().Unix()
		}
		HashSetWriteToFile(converted, hashFile)
	}
}

func ConvertAllAgentsFromFileToDB(db *sql.DB) {
	content, e := os.ReadFile("./agents/agents.json")
	if DidFail(e, "read agents.json file") {
		return
	}

	var result []NewsAgent
	e = json.Unmarshal(content, &result)
	if DidFail(e, "unmarshal news agents") {
		return
	}

	for i := range result {
		agent := result[i]
		hashFile := fmt.Sprintf("./agents/%d.gob", agent.ID)
		visited := HashSetFromFile[int64](hashFile)
		for k := range visited {
			DBAgentPathInsert(db, agent.ID, k)
		}
	}
}

func NATrimOldUrlsFromNewAgentHashSets(db *sql.DB) {
	fmt.Printf("[AGENTS] Start Cleaning Article URLs\n")
	DBAgentPathRemoveOld(db, 12)
	fmt.Printf("[AGENTS] Finish Cleaning Article URLs\n")
}

func NAWriteAllNewsAgents(agents []NewsAgent) {
	file, e := json.Marshal(agents)
	if DidFail(e, "marshal agents") {
		return
	}

	e = os.WriteFile("./agents/agents.json", file, 0644)
	if DidFail(e, "write agents to file") {
		return
	}
}

func NAUpdateNewsAgentWithID(db *sql.DB, id int64) WebsiteScrapings {
	var agent NewsAgent
	for _, a := range agents {
		if a.ID == id {
			agent = a
			break
		}
	}
	wg := new(sync.WaitGroup)
	return NAUpdateNewsAgent(db, agent, wg)
}

func NAUpdateAllNewsAgent(db *sql.DB, before time.Duration) []WebsiteScrapings {
	result := []WebsiteScrapings{}
	wg := new(sync.WaitGroup)
	m := sync.Mutex{}
	for i := range agents {
		k := i
		a := agents[i]
		if a.LastUpdate.Before(utc().Add(before)) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				fmt.Printf("[AGENTS] Fetching %s\n", a.Origin)
				scrape := NAUpdateNewsAgent(db, a, wg)
				m.Lock()
				result = append(result, scrape)
				agents[k].LastUpdate = utc()
				m.Unlock()
			}()
		}
	}
	wg.Wait()
	NAWriteAllNewsAgents(agents)
	fmt.Printf("[AGENTS] Done Fetching Articles\n")
	return result
}

func NAUpdateNewsAgent(db *sql.DB, agent NewsAgent, wg *sync.WaitGroup) WebsiteScrapings {
	if agent.ID == 0 {
		return WebsiteScrapings{}
	}
	scraping := NAScrapeWebsite(agent.Origin)

	if scraping.Type == "article" {
		NACreatePost(db, agent.ID, agent.Origin, agent.Locale, scraping, true)
	}
	baseURL, e := nurl.Parse(agent.Origin)
	if DidFail(e, "invalid origin url") {
		return WebsiteScrapings{}
	}

	createPosts := func(allUrls []string, id int64, origin string) {
		unlock := agentsMutex.Lock(origin)
		defer unlock()
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
			if !DBAgentPathExists(db, id, canURLString) {
				DBAgentPathInsert(db, id, canURLString)
				subScraping := NAScrapeWebsite(urlString)
				if subScraping.Type == "article" {
					NACreatePost(db, agent.ID, urlString, agent.Locale, subScraping, true)
				}
			}
		}
	}

	if len(scraping.URLs) > 0 {
		go createPosts(scraping.URLs, agent.ID, agent.Origin)
	}
	for i := range agent.Subs {
		sub := agent.Subs[i]
		fullUrl := agent.Origin + sub
		subScrape := NAScrapeWebsite(fullUrl)
		if len(subScrape.URLs) == 0 {
			continue
		}
		var offset = time.Second * time.Duration(i+1) * 2
		anon := func() {
			defer wg.Done()
			createPosts(subScrape.URLs, agent.ID, agent.Origin)
		}
		wg.Add(1)
		time.AfterFunc(offset, anon)
	}

	return scraping
}

func CanonicalURL(a string) string {
	x := strings.TrimPrefix(a, "https://")
	x = strings.TrimPrefix(x, "http://")
	x = strings.TrimPrefix(x, "www.")
	x = strings.TrimSuffix(x, "/")
	x = strings.TrimSuffix(x, "/index.html")
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
	englishWordTagCount := 0
	image := ""
	description := ""
	authors := []string{}
	contentType := ""
	modTime := ""
	pubTime := ""
	locale := ""
	language := ""
	bodyText := ""

	getHTMLNodes(node, false, func(n *html.Node) bool {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "meta":
				properties := getValForAttr(n, "property")
				names := getValForAttr(n, "name")
				if names["og:title"] || properties["og:title"] {
					metaTitleNode = n
				} else if names["article:tag"] || properties["article:tag"] {
					tagList := tagFormat(getFirstValForAttr(n, "content"))
					tags = append(tags, tagList...)
					englishWordTagCount += countEnglishTagWords(tagList)
				} else if names["og:image"] || properties["og:image"] {
					image = getFirstValForAttr(n, "content")
				} else if names["og:description"] || properties["og:description"] {
					description = getFirstValForAttr(n, "content")
				} else if names["article:author"] || properties["article:author"] {
					authors = append(authors, getFirstValForAttr(n, "content"))
				} else if names["article:section"] || properties["article:section"] {
					tagList := tagFormat(getFirstValForAttr(n, "content"))
					tags = append(tags, tagList...)
					englishWordTagCount += countEnglishTagWords(tagList)
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
					tagList := tagFormat(getFirstValForAttr(n, "content"))
					tags = append(tags, tagList...)
					englishWordTagCount += countEnglishTagWords(tagList)
				}
			case "a":
				links = append(links, getFirstValForAttr(n, "href"))
			case "body":
				if englishWordTagCount < 3 {
					bodyText = plainTextFromNode(n)
				}
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

	if len(tags) > 5 {
		tags = tags[:5]
	}

	if len(bodyText) > 0 {
		foundTags := findKeywords(title+" "+description, bodyText)
		if len(foundTags) > 5 {
			foundTags = foundTags[:5]
		}
		tags = append(tags, foundTags...)
	}

	return WebsiteScrapings{title, description, links, tags, authors, image, contentType, locale, language, date}
}

func NACreatePost(db *sql.DB, userId int64, url string, locale string, scrape WebsiteScrapings, store bool) PostResult {
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
		createPostsPartitionTable(db, scrape.Date.Year())
	}

	tags := scrape.Tags
	comps := strings.Split(url, "/")
	limit := 5 // limit url tags to max `limit` categories
	for _, comp := range comps {
		t := strings.ToLower(comp)
		if _, found := tagsOnboarding[t]; found {
			tags = append(tags, t)
		}
		limit -= 1
		if limit <= 0 {
			break
		}
	}
	tags = makeUnique(tags)
	reverse(tags)

	var result PostResult
	if store {
		result = DBCreatePost(db, userId, body, scrape.Date, tags, []string{}, locale)
	} else {
		user, _ := DBGetUser(db, userId, "")
		author := UserProfile{user.ID, user.Name, user.RegisterDate, user.Upvotes, user.Downvotes, int64(user.Investment), false, 0, 0, 0}
		tagObjs := DBCreateTags(db, tags)
		tagIds := []string{}
		for _, tag := range tagObjs {
			tagIds = append(tagIds, fmt.Sprintf("%d", tag.ID))
		}
		currentTime := utc()
		result = PostResult{0, author, body, tagIds, currentTime, []string{}, 0, 0, 0, false, currentTime, 0, 0, 0, 0, 0}
	}
	return result
}

func NAReadAllTagsOnboarding(db *sql.DB) map[string]int64 {
	raw := []string{
		"money", "crime", "energy", "health", "ufc",
		"basketball", "travel", "cricket", "golf",
		"baseball", "snooker", "boxing", "china",
		"tennis", "athletics", "europe", "tv",
		"environment", "football", "family", "india",
		"f1", "africa", "film", "politics",
		"gaming", "world", "finance", "business",
		"us", "tech", "uk", "americas",
		"news", "darts", "science", "rugby",
		"racing", "middle-east", "weird-news", "dieting",
		"celebrity", "sport", "lifestyle", "asia",
		"movies", "asia-pacific", "sex", "weird",
		"technology",
	}
	tags := DBCreateTags(db, raw)
	result := map[string]int64{}
	for _, tag := range tags {
		result[tag.Name] = tag.ID
	}
	return result
}

func NARegisterHourlyUpdates(db *sql.DB) {
	fmt.Printf("[AGENTS] Fetching Articles\n")
	time.AfterFunc(0, func() { NAUpdateAllNewsAgent(db, time.Minute*30) })
	time.AfterFunc(time.Hour, func() { NARegisterHourlyUpdates(db) })
}

func NARegisterWeeklyCleanUp(db *sql.DB) {
	time.AfterFunc(0, func() {
		NATrimOldUrlsFromNewAgentHashSets(db)
		DBDeleteDuplicateAgentPosts(db)
		// DBClearTrashedContent(db)
	})
	time.AfterFunc(time.Hour*24*7, func() { NARegisterWeeklyCleanUp(db) })
}
