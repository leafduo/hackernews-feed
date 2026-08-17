package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/feeds"
)

func TestFeedItemURLsForHackerNewsPosts(t *testing.T) {
	first := Item{ID: 49331033}
	second := Item{ID: 49331034}

	firstID, firstLink := feedItemURLs(first)
	secondID, secondLink := feedItemURLs(second)

	if firstID != "https://news.ycombinator.com/item?id=49331033" {
		t.Fatalf("unexpected first ID: %q", firstID)
	}
	if secondID != "https://news.ycombinator.com/item?id=49331034" {
		t.Fatalf("unexpected second ID: %q", secondID)
	}
	if firstID == secondID {
		t.Fatalf("different Hacker News items have the same ID: %q", firstID)
	}
	if firstLink != firstID || secondLink != secondID {
		t.Fatalf("self-post links do not match their IDs: %q, %q", firstLink, secondLink)
	}
}

func TestFeedItemURLsForRepeatedExternalURL(t *testing.T) {
	const externalURL = "https://example.com/article"
	firstID, firstLink := feedItemURLs(Item{ID: 49331033, URL: externalURL})
	secondID, secondLink := feedItemURLs(Item{ID: 49331034, URL: externalURL})

	if firstID == secondID {
		t.Fatalf("different Hacker News submissions have the same ID: %q", firstID)
	}
	if firstLink != externalURL || secondLink != externalURL {
		t.Fatalf("external links changed: %q, %q", firstLink, secondLink)
	}
}

func TestAtomSerializationUsesExplicitHackerNewsIDs(t *testing.T) {
	created := time.Date(2026, time.August, 17, 0, 0, 0, 0, time.UTC)
	items := []Item{{ID: 49331033}, {ID: 49331034}}
	feedItems := make([]*feeds.Item, 0, len(items))
	for _, item := range items {
		id, link := feedItemURLs(item)
		feedItems = append(feedItems, &feeds.Item{
			Title:   "HN post",
			Link:    &feeds.Link{Href: link},
			Id:      id,
			Created: created,
			Updated: created,
		})
	}

	feed := feeds.Feed{
		Title:   "Hackernews customized feed",
		Link:    &feeds.Link{Href: "https://news.ycombinator.com/"},
		Created: created,
		Items:   feedItems,
	}
	atom, err := feed.ToAtom()
	if err != nil {
		t.Fatalf("serialize Atom feed: %v", err)
	}

	for _, item := range items {
		expectedID := "<id>" + hackerNewsItemURL(item.ID) + "</id>"
		if !strings.Contains(atom, expectedID) {
			t.Errorf("Atom feed does not contain %q", expectedID)
		}
	}
	if strings.Contains(atom, "tag:news.ycombinator.com") {
		t.Fatalf("Atom feed contains a generated, query-less Hacker News ID: %s", atom)
	}
}

type fakeHackerNewsAPI struct {
	storyIDs []int64
	items    map[int64]Item
	errors   map[int64]error
}

func (api fakeHackerNewsAPI) ListTopStories(context.Context) ([]int64, error) {
	return api.storyIDs, nil
}

func (api fakeHackerNewsAPI) GetItem(_ context.Context, id int64) (Item, error) {
	if err := api.errors[id]; err != nil {
		return Item{}, err
	}
	return api.items[id], nil
}

func TestDownloadItemsPreservesOrderAndSkipsFailures(t *testing.T) {
	api := fakeHackerNewsAPI{
		storyIDs: []int64{30, 20, 10},
		items: map[int64]Item{
			30: {ID: 30},
			10: {ID: 10},
		},
		errors: map[int64]error{20: errors.New("unavailable")},
	}

	items, err := downloadItems(context.Background(), api)
	if err != nil {
		t.Fatalf("download items: %v", err)
	}
	if len(items) != 2 || items[0].ID != 30 || items[1].ID != 10 {
		t.Fatalf("unexpected items: %#v", items)
	}
}

func TestWriteGeneratedFilesUsesOutputDirectory(t *testing.T) {
	outputDir := t.TempDir()
	updatedAt := time.Date(2026, time.August, 18, 1, 2, 3, 0, time.UTC)
	if err := writeGeneratedFiles(outputDir, "<feed></feed>", updatedAt); err != nil {
		t.Fatalf("write generated files: %v", err)
	}

	atom, err := os.ReadFile(filepath.Join(outputDir, "hn-feed.atom"))
	if err != nil {
		t.Fatalf("read Atom output: %v", err)
	}
	if string(atom) != "<feed></feed>" {
		t.Fatalf("unexpected Atom output: %q", atom)
	}
	timestamp, err := os.ReadFile(filepath.Join(outputDir, "last_updated.txt"))
	if err != nil {
		t.Fatalf("read timestamp output: %v", err)
	}
	if string(timestamp) != updatedAt.String() {
		t.Fatalf("unexpected timestamp: %q", timestamp)
	}
	temporaryFiles, err := filepath.Glob(filepath.Join(outputDir, ".*.tmp-*"))
	if err != nil {
		t.Fatalf("find temporary files: %v", err)
	}
	if len(temporaryFiles) != 0 {
		t.Fatalf("temporary files were not cleaned up: %v", temporaryFiles)
	}
}
