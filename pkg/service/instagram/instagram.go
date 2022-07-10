package instagram

import (
	"context"
	"fmt"
	"net/http"

	"daunrodo/internal/entity"
)

type InstagramCrawler interface {
	Download(context.Context, string) ([]entity.File, error)
}

type Instagram struct {
	URL        string
	HTTPClient *http.Client
}

// New creates instagram crawler
func New(c *http.Client) *Instagram {
	return &Instagram{HTTPClient: c}
}

func (i *Instagram) Download(ctx context.Context, link string) ([]entity.File, error) {
	link = i.parse(link)

	req, err := http.NewRequest(http.MethodGet, link, nil)
	if err != nil {
		return nil, fmt.Errorf("NewRequest: %w", err)
	}

	res, err := i.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do: %w", err)
	}
	defer res.Body.Close()

	files := make([]entity.File, 0)
	files = append(files, entity.File{URL: res.Request.URL.String()})

	return files, nil
}

func (i *Instagram) parse(link string) string {
	return fmt.Sprintf("%v%v", link, "media/?size=l")
}
