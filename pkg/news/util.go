package news

import (
	"net/http"
	"slices"
	"strings"

	"golang.org/x/net/html"
)

// getHttpResponseBody fetches the HTML content of a given URL and returns the parsed HTML node.
func getHttpResponseBody(url string) (*html.Node, error) {
	// Make the GET request
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Use the html package to parse the response body from the request
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// extractText extracts the text content from an HTML node, ignoring any nested tags.
func extractText(n *html.Node) string {
	if n.Type == html.TextNode {
		return strings.TrimSpace(n.Data)
	}

	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		text := extractText(c)
		if text != "" {
			if sb.Len() > 0 {
				sb.WriteString(" ")
			}
			sb.WriteString(text)
		}
	}
	return strings.TrimSpace(sb.String())
}

// extractFirstAnchor extracts the first anchor tag's href and text content from an HTML node.
func extractFirstAnchor(n *html.Node) (href, text string, found bool) {
	if n.Type == html.ElementNode && n.Data == "a" {
		for _, attr := range n.Attr {
			if attr.Key == "href" {
				href = attr.Val
			}
		}
		text = extractText(n)
		return href, text, true
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if href, text, found := extractFirstAnchor(c); found {
			return href, text, true
		}
	}
	return "", "", false
}

// hasClass checks if an HTML node has a specific class attribute.
func hasClass(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			classes := strings.Fields(attr.Val)
			if slices.Contains(classes, class) {
				return true
			}

		}
	}
	return false
}

// isUnderClass checks if an HTML node is under a ancestor with a specific class attribute.
func isUnderClass(n *html.Node, class string) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if hasClass(p, class) {
			return true
		}
	}
	return false
}
