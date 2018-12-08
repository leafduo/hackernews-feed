package hackernews

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type HackerNewsAPI interface {
	ListTopStories(ctx context.Context) ([]int64, error)
	GetItem(ctx context.Context, id int64) (Item, error)
}

func NewHackerNewsAPI() HackerNewsAPI {
	return basicHackerNewsAPI{}
}

type basicHackerNewsAPI struct {
}

func (api basicHackerNewsAPI) ListTopStories(ctx context.Context) ([]int64, error) {
	resp, err := http.Get("https://hacker-news.firebaseio.com/v0/topstories.json")
	if err != nil {
		return nil, err
	}

	storyIDs := make([]int64, 0)
	err = json.NewDecoder(resp.Body).Decode(&storyIDs)
	if err != nil {
		return nil, err
	}

	return storyIDs, nil
}

func (api basicHackerNewsAPI) GetItem(ctx context.Context, id int64) (Item, error) {
	resp, err := http.Get(fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json", id))
	if err != nil {
		return Item{}, err
	}

	item := Item{}
	err = json.NewDecoder(resp.Body).Decode(&item)
	if err != nil {
		return Item{}, err
	}

	return item, nil
}
