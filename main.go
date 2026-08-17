package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/feeds"
)

const concurrency = 10
const downloadItemLimit = 50
const scoreThreshold = 50
const requestTimeout = 10 * time.Second
const refreshInterval = 20 * time.Minute

var articleHTTPClient = &http.Client{Timeout: requestTimeout}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	run(ctx, outputDirectory())
}

func outputDirectory() string {
	if outputDir := os.Getenv("HN_FEED_OUTPUT_DIR"); outputDir != "" {
		return outputDir
	}
	return "."
}

func run(ctx context.Context, outputDir string) {
	for {
		if err := doWork(ctx, outputDir); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			slog.Error("feed generation failed", "error", err)
		}

		slog.Info("sleeping before next refresh", "duration", refreshInterval)
		select {
		case <-ctx.Done():
			return
		case <-time.After(refreshInterval):
		}
	}
}

func doWork(ctx context.Context, outputDir string) error {
	slog.Info("generating Hacker News feed")
	items, err := downloadItems(ctx, NewHackerNewsAPI())
	if err != nil {
		return fmt.Errorf("download Hacker News items: %w", err)
	}

	filteredItems := filterScoreAbove(items, scoreThreshold)
	feed, err := generateFeed(ctx, filteredItems)
	if err != nil {
		return fmt.Errorf("generate feed: %w", err)
	}

	atom, err := feed.ToAtom()
	if err != nil {
		return fmt.Errorf("serialize Atom feed: %w", err)
	}
	if err := writeGeneratedFiles(outputDir, atom, time.Now()); err != nil {
		return err
	}

	slog.Info("wrote Atom feed", "items", len(feed.Items), "output_dir", outputDir)
	return nil
}

func downloadItems(ctx context.Context, api HackerNewsAPI) ([]Item, error) {
	storyIDs, err := api.ListTopStories(ctx)
	if err != nil {
		return nil, err
	}

	itemCount := min(downloadItemLimit, len(storyIDs))
	type itemTask struct {
		index int
		id    int64
	}
	type itemResult struct {
		index int
		item  Item
	}

	tasks := make(chan itemTask, itemCount)
	results := make(chan itemResult, itemCount)
	workerCount := min(concurrency, itemCount)

	var wg sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				itemCtx, cancel := context.WithTimeout(ctx, requestTimeout)
				item, err := api.GetItem(itemCtx, task.id)
				cancel()
				if err != nil {
					slog.Warn("item metadata request failed", "item_id", task.id, "error", err)
					continue
				}

				select {
				case results <- itemResult{index: task.index, item: item}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	for index, storyID := range storyIDs[:itemCount] {
		tasks <- itemTask{index: index, id: storyID}
	}
	close(tasks)
	wg.Wait()
	close(results)

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	orderedItems := make([]Item, itemCount)
	present := make([]bool, itemCount)
	for result := range results {
		orderedItems[result.index] = result.item
		present[result.index] = true
	}

	items := make([]Item, 0, itemCount)
	for index, item := range orderedItems {
		if present[index] {
			slog.Info("got item metadata", "item_id", item.ID, "title", item.Title, "score", item.Score)
			items = append(items, item)
		}
	}
	return items, nil
}

func filterScoreAbove(items []Item, threshold int) []Item {
	filteredItems := make([]Item, 0, len(items))
	for _, item := range items {
		if item.Score >= threshold {
			filteredItems = append(filteredItems, item)
		}
	}
	return filteredItems
}

func hackerNewsItemURL(itemID int64) string {
	return fmt.Sprintf("https://news.ycombinator.com/item?id=%d", itemID)
}

func feedItemURLs(item Item) (id string, link string) {
	id = hackerNewsItemURL(item.ID)
	if item.URL != "" {
		return id, item.URL
	}
	return id, id
}

func generateFeedItem(ctx context.Context, item Item) (feeds.Item, error) {
	content, author := "", ""
	if item.URL != "" {
		var err error
		content, author, err = fetchArticle(ctx, articleHTTPClient, item.URL)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return feeds.Item{}, err
			}
			slog.Warn("readability extraction failed", "item_id", item.ID, "url", item.URL, "error", err)
		}
	}

	id, link := feedItemURLs(item)
	return feeds.Item{
		Title:   fmt.Sprintf("%s (%d)", item.Title, item.Score),
		Content: content,
		Author:  &feeds.Author{Name: author},
		Link:    &feeds.Link{Href: link},
		Id:      id,
		Created: time.Unix(item.Time, 0),
		Updated: time.Now(),
	}, nil
}

func generateFeed(ctx context.Context, items []Item) (feeds.Feed, error) {
	feedItemMap := make(map[int64]feeds.Item, len(items))
	tasks := make(chan Item, len(items))
	workerCount := min(concurrency, len(items))

	var lock sync.Mutex
	var wg sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range tasks {
				feedItem, err := generateFeedItem(ctx, item)
				if err != nil {
					continue
				}
				slog.Info("generated feed item", "item_id", item.ID, "title", feedItem.Title)

				lock.Lock()
				feedItemMap[item.ID] = feedItem
				lock.Unlock()
			}
		}()
	}

	for _, item := range items {
		tasks <- item
	}
	close(tasks)
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return feeds.Feed{}, err
	}

	feedItems := make([]*feeds.Item, 0, len(items))
	for _, item := range items {
		if feedItem, ok := feedItemMap[item.ID]; ok {
			feedItem := feedItem
			feedItems = append(feedItems, &feedItem)
		}
	}

	now := time.Now()
	return feeds.Feed{
		Title:       "Hackernews customized feed",
		Link:        &feeds.Link{Href: "https://news.ycombinator.com/"},
		Description: "Hackernews customized feed",
		Author:      &feeds.Author{Name: "leafduo", Email: "leafduo@gmail.com"},
		Created:     now,
		Updated:     now,
		Items:       feedItems,
	}, nil
}

func writeGeneratedFiles(outputDir, atom string, updatedAt time.Time) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := writeFileAtomically(filepath.Join(outputDir, "hn-feed.atom"), []byte(atom)); err != nil {
		return fmt.Errorf("write Atom feed: %w", err)
	}
	if err := writeFileAtomically(filepath.Join(outputDir, "last_updated.txt"), []byte(updatedAt.String())); err != nil {
		return fmt.Errorf("write update timestamp: %w", err)
	}
	return nil
}

func writeFileAtomically(filename string, data []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(filename), "."+filepath.Base(filename)+".tmp-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer func() {
		temporary.Close()
		os.Remove(temporaryName)
	}()

	if err := temporary.Chmod(0o644); err != nil {
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, filename)
}
