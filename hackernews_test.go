package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHackerNewsAPIRequests(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/topstories.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[123,456]`))
	})
	mux.HandleFunc("/item/123.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":123,"time":456,"score":78,"url":"https://example.com","title":"Example"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	api := newHackerNewsAPI(server.Client(), server.URL)
	storyIDs, err := api.ListTopStories(context.Background())
	if err != nil {
		t.Fatalf("list top stories: %v", err)
	}
	if len(storyIDs) != 2 || storyIDs[0] != 123 || storyIDs[1] != 456 {
		t.Fatalf("unexpected story IDs: %v", storyIDs)
	}
	item, err := api.GetItem(context.Background(), 123)
	if err != nil {
		t.Fatalf("get item: %v", err)
	}
	if item.ID != 123 || item.Title != "Example" || item.URL != "https://example.com" {
		t.Fatalf("unexpected item: %#v", item)
	}
}

func TestHackerNewsAPIRejectsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "try again later", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	api := newHackerNewsAPI(server.Client(), server.URL)
	if _, err := api.ListTopStories(context.Background()); err == nil {
		t.Fatal("expected an HTTP status error")
	}
}

func TestHackerNewsAPIHonorsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	api := newHackerNewsAPI(http.DefaultClient, "https://example.com")
	_, err := api.ListTopStories(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}
