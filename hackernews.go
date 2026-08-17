package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const hackerNewsAPIBaseURL = "https://hacker-news.firebaseio.com/v0"

type HackerNewsAPI interface {
	ListTopStories(ctx context.Context) ([]int64, error)
	GetItem(ctx context.Context, id int64) (Item, error)
}

type basicHackerNewsAPI struct {
	client  *http.Client
	baseURL string
}

func NewHackerNewsAPI() HackerNewsAPI {
	return newHackerNewsAPI(&http.Client{Timeout: requestTimeout}, hackerNewsAPIBaseURL)
}

func newHackerNewsAPI(client *http.Client, baseURL string) basicHackerNewsAPI {
	return basicHackerNewsAPI{
		client:  client,
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

func (api basicHackerNewsAPI) ListTopStories(ctx context.Context) ([]int64, error) {
	storyIDs := make([]int64, 0)
	if err := api.getJSON(ctx, "/topstories.json", &storyIDs); err != nil {
		return nil, err
	}
	return storyIDs, nil
}

func (api basicHackerNewsAPI) GetItem(ctx context.Context, id int64) (Item, error) {
	item := Item{}
	if err := api.getJSON(ctx, fmt.Sprintf("/item/%d.json", id), &item); err != nil {
		return Item{}, err
	}
	if item.ID == 0 {
		return Item{}, fmt.Errorf("item %d returned no data", id)
	}
	return item, nil
}

func (api basicHackerNewsAPI) getJSON(ctx context.Context, path string, destination any) error {
	requestURL := api.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := api.client.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", requestURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("GET %s: unexpected status %s: %s", requestURL, resp.Status, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(destination); err != nil {
		return fmt.Errorf("decode %s: %w", requestURL, err)
	}
	return nil
}
