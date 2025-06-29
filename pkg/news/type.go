package news

type RelatedArticle struct {
	Title string
	URL   string
}

type Article struct {
	Title           string
	URL             string
	RelatedArticles []RelatedArticle
}
