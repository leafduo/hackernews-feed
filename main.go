package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"sync"
	"time"

	readability "github.com/RadhiFadlillah/go-readability"
	"github.com/gorilla/feeds"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const concurrency = 10
const downloadItemLimit = 50
const scoreThreshold = 50
const timeout = 10 * time.Second

func main() {
	viper.SetEnvPrefix("hn_feed")
	viper.BindEnv("output_dir")
	viper.SetDefault("output_dir", ".")

	for {
		doWork()
		logrus.Infof("Sleeping")
		time.Sleep(20 * time.Minute)
	}
}

func doWork() {
	logrus.Infof("Start to generate hackernews feed")
	ctx := context.Background()
	itemChannel := make(chan Item)
	go downloadItems(ctx, itemChannel)

	items := make([]Item, 0)
	for item := range itemChannel {
		logrus.WithFields(logrus.Fields{
			"Title": item.Title,
			"Score": item.Score,
			"Link":  item.URL,
			"ID":    item.ID,
		}).Infof("Got metadata for item %d", item.ID)
		items = append(items, item)
	}
	filteredItems := filterScoreAbove(items, scoreThreshold)

	feed, err := generateFeed(ctx, filteredItems)
	if err != nil {
		panic(err)
	}

	outputDir := viper.GetString("output_dir")
	file, err := os.Create(path.Join(path.Dir(outputDir), "hn-feed.atom"))
	if err != nil {
		panic(err)
	}
	atom, err := feed.ToAtom()
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(file, "%s", atom)
	logrus.Infof("Wrote atom feed")

	updatedTimestmapFile, err := os.Create(path.Join(path.Dir(outputDir), "last_updated.txt"))
	fmt.Fprintf(updatedTimestmapFile, "%v", time.Now())
}

func downloadItems(ctx context.Context, itemChannel chan Item) {
	api := NewHackerNewsAPI()
	storyIDs, err := api.ListTopStories(ctx)
	if err != nil {
		panic(err)
	}
	tasks := make(chan int64)
	wg := sync.WaitGroup{}
	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for storyID := range tasks {
				itemCtx, cancel := context.WithTimeout(ctx, timeout)
				defer cancel()
				item, err := api.GetItem(itemCtx, storyID)
				if err != nil {
					logrus.WithError(err).Errorf("Item metadata request failed")
				}
				itemChannel <- item
			}
		}()
	}

	for _, storyID := range storyIDs[0:downloadItemLimit] {
		tasks <- storyID
	}
	close(tasks)
	wg.Wait()
	close(itemChannel)
}

func filterScoreAbove(items []Item, threshold int) []Item {
	filteredItems := make([]Item, 0)
	for _, item := range items {
		if item.Score >= threshold {
			filteredItems = append(filteredItems, item)
		}
	}

	return filteredItems
}

func generateFeedItem(ctx context.Context, item Item) (feeds.Item, error) {
	parsedURL, _ := url.Parse(item.URL)
	article, err := readability.FromURL(parsedURL, timeout) // WTF? Doesn't support context?
	if err != nil {
		// If error occurs, we will leave content empty
		article = readability.Article{}
		logrus.WithError(err).
			WithField("content", article.RawContent).
			WithField("item", item).
			Errorf("Redability failed.")
	}

	var link string
	if len(item.URL) > 0 {
		link = item.URL
	} else {
		link = fmt.Sprintf("https://news.ycombinator.com/item?id=%d", item.ID)
	}
	feedItem := feeds.Item{
		Title:   fmt.Sprintf("%s (%d)", item.Title, item.Score),
		Content: article.RawContent,
		Author:  &feeds.Author{Name: article.Meta.Author},
		Link:    &feeds.Link{Href: link},
		Id:      item.URL, // For compatability.
		Created: time.Unix(item.Time, 0),
		Updated: time.Now(),
	}
	return feedItem, nil
}

func generateFeed(ctx context.Context, items []Item) (feeds.Feed, error) {
	feedItemMap := make(map[int64]feeds.Item, 0)
	lock := sync.Mutex{}

	wg := sync.WaitGroup{}
	tasks := make(chan Item)
	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range tasks {
				feedItem, err := generateFeedItem(ctx, item)
				if err != nil {
					continue
				}
				logrus.Infof("Got feed item %s", feedItem.Title)

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

	feedItems := make([]*feeds.Item, 0)
	for _, item := range items {
		if feedItem, ok := feedItemMap[item.ID]; ok {
			feedItems = append(feedItems, &feedItem)
		}
	}

	now := time.Now()
	feed := feeds.Feed{
		Title:       "Hackernews customized feed",
		Link:        &feeds.Link{Href: "https://news.ycombinator.com/"},
		Description: "Hackernews customized feed",
		Author:      &feeds.Author{Name: "leafduo", Email: "leafduo@gmail.com"},
		Created:     now,
	}

	feed.Items = feedItems

	return feed, nil
}
