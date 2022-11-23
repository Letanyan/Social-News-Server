package main

import (
	"bufio"
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/bbalet/stopwords"
	"golang.org/x/net/html"
	"jaytaylor.com/html2text"
)

func init() {
	englishWords = map[string]bool{}
	file, e := os.Open("./words/english_words.txt")
	if !DidFail(e, "read english words txt file") {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			englishWords[scanner.Text()] = true
		}
		e = scanner.Err()
		DidFail(e, "scanning english words file")
	}

}

func getValForAttr(node *html.Node, key string) map[string]bool {
	result := map[string]bool{}
	for _, attr := range node.Attr {
		if attr.Key == key {
			result[attr.Val] = true
		}
	}
	return result
}

func getFirstValForAttr(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func plainTextFromNode(node *html.Node) string {
	text, err := html2text.FromHTMLNode(node, html2text.Options{PrettyTables: false, OmitLinks: true, TextOnly: true})
	if err != nil {
		println(err.Error())
		return ""
	}
	return text
}

func getHTMLNodes(node *html.Node, skipBody bool, condition func(*html.Node) bool) []*html.Node {
	result := []*html.Node{}
	var process func(*html.Node)
	process = func(n *html.Node) {
		if condition(n) {
			result = append(result, n)
		}

		if skipBody && n.Type == html.ElementNode && n.Data == "body" {
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			process(c)
		}
	}
	process(node)
	return result
}

func wordBag(text string) map[string]int {
	result := map[string]int{}
	inWord := false
	word := ""
	lastWord := ""
	lastLastWord := ""
	for _, c := range text {
		if unicode.IsLetter(c) {
			inWord = true
			word += string(c)
		} else if inWord {
			lowWord := strings.ToLower(word)
			stopWord := stopwords.CleanString(lowWord, "en", false)
			stopWord = strings.TrimSpace(stopWord)
			if len(stopWord) > 2 {
				result[stopWord] += 1
				if len(lastWord) > 2 {
					result[lastWord+" "+stopWord] += 2
					if len(lastLastWord) > 2 {
						result[lastLastWord+" "+lastWord+" "+stopWord] += 3
					}
				}
			}

			lastLastWord = lastWord
			lastWord = stopWord
			word = ""
			inWord = false
		}
	}
	return result
}

type KeyValue struct {
	key   string
	value int
}

func orderedMapByValue(keyValues map[string]int) []KeyValue {
	result := []KeyValue{}
	for k, v := range keyValues {
		result = append(result, KeyValue{k, v})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].value > result[j].value })
	return result
}

func findKeywords(title string, text string) []string {
	bodyBag := wordBag(text)
	titleBag := wordBag(title)

	topBody := orderedMapByValue(bodyBag)

	limit := 200
	result := []string{}
	for _, kv := range topBody {
		if _, hasKey := titleBag[kv.key]; hasKey {
			delete(titleBag, kv.key)
			for k := range titleBag {
				if strings.Contains(k, kv.key) {
					delete(titleBag, k)
				}
			}
			a := strings.SplitN(kv.key, " ", 3)
			if len(a) > 0 {
				w := strings.TrimSpace(a[0])
				if len(w) > 0 {
					delete(titleBag, w)
				}
			}
			if len(a) > 1 {
				w := strings.TrimSpace(a[1])
				if len(w) > 0 {
					delete(titleBag, w)
				}
			}
			if len(a) > 2 {
				w := strings.TrimSpace(a[2])
				if len(w) > 0 {
					delete(titleBag, w)
				}
			}
			result = append(result, kv.key)
		}
		limit -= 1
		if limit <= 0 {
			break
		}
	}

	return result
}

func countEnglishTagWords(tags []string) int {
	result := 0
	for _, tag := range tags {
		words := strings.Split(tag, " ")
		for _, word := range words {
			if len(word) > 3 && englishWords[word] {
				result += 1
				break
			}
		}
	}
	return result
}
