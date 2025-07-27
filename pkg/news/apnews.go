package news

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"golang.org/x/net/html"
)

type APNews struct {
	Articles []Article
}

// NewAPNews creates a new instance of APNews by fetching the latest articles from the AP News website.
func NewAPNews() (*APNews, error) {
	url := "https://apnews.com"
	doc, err := getHttpResponseBody(url)
	if err != nil {
		return nil, err
	}

	a := APNews{
		Articles: []Article{},
	}
	a.Articles = a.extractArticles(doc)

	return &a, nil
}

// ListArticles lists all articles with their titles and related articles.
func (a *APNews) ListArticles() error {
	if a.Articles == nil || len(a.Articles) == 0 {
		return fmt.Errorf("no articles available")
	}

	writer := tabwriter.NewWriter(
		os.Stdout, 0, 2, 4, ' ', 0,
	)
	_, err := writer.Write([]byte("Number\tTitle\n"))
	if err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	for articleIndex, article := range a.Articles {
		if article.URL == "" {
			continue
		}
		_, err := fmt.Fprintf(writer, "%d\t%s\n", articleIndex+1, article.Title)
		if err != nil {
			return fmt.Errorf("failed to write article data: %w", err)
		}
		for relatedIndex, relatedArticle := range article.RelatedArticles {
			if relatedArticle.URL == "" {
				continue
			}
			_, err := fmt.Fprintf(
				writer,
				" %d-%d\t%s\n",
				articleIndex+1,
				relatedIndex+1,
				relatedArticle.Title,
			)
			if err != nil {
				return fmt.Errorf("failed to write related article data: %w", err)
			}
		}
	}
	err = writer.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}
	return nil
}

// RetrieveArticle retrieves the content of an article by its index.
func (a *APNews) RetrieveArticle(index string) (string, error) {
	if index == "" {
		return "", fmt.Errorf("index cannot be empty")
	}

	if a.Articles == nil || len(a.Articles) == 0 {
		return "", fmt.Errorf("no articles available")
	}

	var articleUrl string
	for articleIndex, article := range a.Articles {
		if strconv.Itoa(articleIndex+1) == index {
			if article.Title == "" || article.URL == "" {
				return "", fmt.Errorf("article at index %s is missing title or URL", index)
			}
			articleUrl = article.URL
			break
		}

		for relatedIndex, relatedArticle := range article.RelatedArticles {
			if (strconv.Itoa(articleIndex+1) + "-" + strconv.Itoa(relatedIndex+1)) == index {
				if relatedArticle.Title == "" || relatedArticle.URL == "" {
					return "", fmt.Errorf(
						"related article at index %s is missing title or URL",
						index,
					)
				}
				articleUrl = relatedArticle.URL
				break
			}
		}
	}
	if articleUrl == "" {
		return "", fmt.Errorf("article with index %s not found", index)
	}

	doc, err := getHttpResponseBody(articleUrl)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve article: %w", err)
	}
	article, err := a.extractArticle(doc)
	if err != nil {
		return "", fmt.Errorf("failed to extract article content: %w", err)
	}
	if article.Title == "" || article.Body == "" {
		return "", fmt.Errorf("article content is empty")
	}
	fullArticleText := fmt.Sprintf("\n# \033]8;;%s\a%s\033]8;;\a\n\n%s",
		articleUrl, article.Title, article.Body)

	return fullArticleText, nil
}

// extractArticle extracts the main content of an article from the given HTML document.
func (a *APNews) extractArticle(doc *html.Node) (*struct {
	Title string
	Body  string
}, error,
) {
	var headerNode *html.Node
	var findHeader func(*html.Node) bool
	findHeader = func(n *html.Node) bool {
		if n.Type == html.ElementNode && hasClass(n, "Page-headline") {
			headerNode = n
			return true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if findHeader(c) {
				return true
			}
		}
		return false
	}

	findHeader(doc)
	if headerNode == nil {
		return nil, fmt.Errorf("header not found")
	}

	var contentBodyNode *html.Node
	var findContentBody func(*html.Node) bool
	findContentBody = func(n *html.Node) bool {
		if n.Type == html.ElementNode && hasClass(n, "RichTextBody") {
			contentBodyNode = n
			return true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if findContentBody(c) {
				return true
			}
		}
		return false
	}

	findContentBody(doc)
	if contentBodyNode == nil {
		return nil, fmt.Errorf("content not found")
	}

	// Write header
	headerText := extractText(headerNode)

	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			tag := c.Data
			text := extractText(c)
			if text == "" {
				continue
			}

			switch tag {
			case "div":
				if hasClass(c, "Infobox") {
					walk(c)
				} else if hasClass(c, "Infobox-items") && c.FirstChild != nil {
					sb.WriteString("## Information\n\n")
					walk(c)
				}
			case "ul":
				walk(c)
			case "li":
				sb.WriteString("- " + text + "\n\n")
			case "p":
				if isUnderClass(c, "Infobox") {
					sb.WriteString("### " + text + "\n\n")
				} else {
					sb.WriteString(text + "\n\n")
				}
			case "h2":
				sb.WriteString("## " + text + "\n\n")
			case "h3":
				sb.WriteString("### " + text + "\n\n")
			case "h4":
				sb.WriteString("#### " + text + "\n\n")
			case "h5":
				sb.WriteString("##### " + text + "\n\n")
			case "h6":
				sb.WriteString("###### " + text + "\n\n")
			}
		}
	}

	walk(contentBodyNode)

	return &struct {
		Title string
		Body  string
	}{
		Title: headerText,
		Body:  sb.String(),
	}, nil
}

// extractArticles extracts articles from the the given HTML document.
func (a *APNews) extractArticles(doc *html.Node) []Article {
	var results []Article

	// Step 1: Find the first TwoColumnContainer7030-container
	var firstContainer *html.Node
	var findContainer func(*html.Node) bool
	findContainer = func(n *html.Node) bool {
		if n.Type == html.ElementNode && hasClass(n, "TwoColumnContainer7030-container") {
			firstContainer = n
			return true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if findContainer(c) {
				return true
			}
		}
		return false
	}
	findContainer(doc)

	if firstContainer == nil {
		return results // not found
	}

	// Step 2: Process all PageListStandardE-items inside the container
	processedContainers := make(map[*html.Node]bool)
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && hasClass(n, "PageListStandardE-items") {
			if processedContainers[n] {
				return
			}
			processedContainers[n] = true

			main, ok := a.findFirstHeadline(n)
			if !ok {
				return
			}

			var more []RelatedArticle
			var searchSecondary func(*html.Node)
			searchSecondary = func(node *html.Node) {
				if hasClass(node, "PageListStandardE-items-secondary") {
					more = append(more, a.collectHeadlines(node)...)
				}
				for c := node.FirstChild; c != nil; c = c.NextSibling {
					searchSecondary(c)
				}
			}
			searchSecondary(n)

			results = append(results, Article{
				Title:           main.Title,
				URL:             main.URL,
				RelatedArticles: more,
			})
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(firstContainer)

	// Step 3: Collect standalone bsp-custom-headline inside the container
	var collectStandalone func(*html.Node)
	seen := make(map[*html.Node]bool)
	collectStandalone = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "bsp-custom-headline" {
			if !isUnderClass(n, "PageListStandardE-items") {
				if seen[n] {
					return
				}
				seen[n] = true
				if href, text, ok := extractFirstAnchor(n); ok {
					results = append(results, Article{
						Title:           text,
						URL:             href,
						RelatedArticles: []RelatedArticle{},
					})
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collectStandalone(c)
		}
	}
	collectStandalone(firstContainer)

	return results
}

func (a *APNews) collectHeadlines(n *html.Node) []RelatedArticle {
	var headlines []RelatedArticle
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "bsp-custom-headline" {
			if href, text, found := extractFirstAnchor(n); found {
				headlines = append(headlines, RelatedArticle{Title: text, URL: href})
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return headlines
}

func (a *APNews) findFirstHeadline(n *html.Node) (RelatedArticle, bool) {
	var result RelatedArticle
	var found bool
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found {
			return
		}
		if n.Type == html.ElementNode && n.Data == "bsp-custom-headline" {
			if href, text, ok := extractFirstAnchor(n); ok {
				result = RelatedArticle{Title: text, URL: href}
				found = true
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return result, found
}
