package news

import (
	"fmt"
)

type News struct {
	ApNews *APNews
}

func (n *News) NewNews(source string) (*News, error) {
	switch source {
	case "apnews":
		apNews, err := NewAPNews()
		if err != nil {
			return nil, fmt.Errorf("error creating APNews: %v", err)
		}
		return &News{ApNews: apNews}, nil
	}
	return nil, fmt.Errorf("unsupported news source: %s", source)
}
