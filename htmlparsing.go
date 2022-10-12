package main

import (
	"golang.org/x/net/html"
)

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

// func setFirstValForAttr(node *html.Node, key string, val string) {
// 	didUpdate := false
// 	for i, attr := range node.Attr {
// 		if attr.Key == key {
// 			node.Attr[i].Val = val
// 			didUpdate = true
// 			break
// 		}
// 	}
// 	if !didUpdate {
// 		attr := html.Attribute{Namespace: "", Key: key, Val: val}
// 		node.Attr = append(node.Attr, attr)
// 	}
// }

// func htmlFromNode(node *html.Node) string {
// 	var buf bytes.Buffer
// 	w := io.Writer(&buf)
// 	html.Render(w, node)
// 	return buf.String()
// }

// func plainTextFromNode(node *html.Node) string {
// 	text, err := html2text.FromHTMLNode(node, html2text.Options{PrettyTables: false, OmitLinks: true, TextOnly: true})
// 	if err != nil {
// 		println(err.Error())
// 		return ""
// 	}
// 	return text
// }

func getHTMLNodes(node *html.Node, skipBody bool, condition func(*html.Node) bool) []*html.Node {
	result := []*html.Node{}
	var process func(*html.Node)
	process = func(n *html.Node) {
		if skipBody && n.Type == html.ElementNode && n.Data == "body" {
			return
		}

		if condition(n) {
			result = append(result, n)
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			process(c)
		}
	}
	process(node)
	return result
}
