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
}

type WebsiteScrapings struct {
	Title       string
	Description string
	URLs        []string
	Tags        []string
	Author      string
	Image       string
	Type        string
}

func NACreateNewsAgent(id int64, name string, origin string) (NewsAgent, error) {
	e := NAAgentExists(name, origin)
	if e != nil {
		return NewsAgent{}, e
	}
	agent := NewsAgent{id, name, origin, utc()}
	agents = append(agents, agent)
	NAWriteAllNewsAgents(agents)
	bloom := NewBloomFilterP(0.99, 100_000_000)
	bloom.Write(fmt.Sprintf("./agents/%d.json", id))
	return agent, nil
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

func NAUpdateAllNewsAgent() []WebsiteScrapings {
	result := []WebsiteScrapings{}
	for _, a := range agents {
		result = append(result, NAUpdateNewsAgent(a))
	}
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
	bloomFile := fmt.Sprintf("./agents/%d.json", agent.ID)
	bloom := NewBloomFilterF(bloomFile)
	defer bloom.Write(bloomFile)
	for _, urlString := range scraping.URLs {
		url, e := nurl.Parse(urlString)
		urlHost := strings.TrimPrefix(url.Hostname(), "www.")
		baseHost := strings.TrimPrefix(baseURL.Hostname(), "www.")
		if e != nil || baseHost != urlHost {
			continue
		}
		if !bloom.Contains(urlString) {
			bloom.Insert(urlString)
			subScraping := NAScrapeWebsite(urlString)
			if subScraping.Type == "article" {
				NACreatePost(agent.ID, urlString, subScraping)
			}
		}
	}

	return scraping
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
	author := ""
	contentType := ""

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
					author = getFirstValForAttr(n, "content")
				} else if names["article:section"] || properties["article:section"] {
					tags = append(tags, tagFormat(getFirstValForAttr(n, "content")))
				} else if names["og:type"] || properties["og:type"] {
					contentType = getFirstValForAttr(n, "content")
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

	return WebsiteScrapings{title, description, links, tags, author, image, contentType}
}

func NACreatePost(userId int64, url string, scrape WebsiteScrapings) {
	body := url + "\n" + scrape.Title

	if len(scrape.Image) > 0 {
		body += "\n" + scrape.Image
	}
	if len(scrape.Description) > 0 {
		body += "\n" + scrape.Description
	}
	if len(scrape.Author) > 0 {
		body += "\n\nBy: " + scrape.Author
	}

	DBCreatePost(mainDB, userId, body, scrape.Tags, []string{})
}
