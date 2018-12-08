package hackernews

// Item represents a hacker new item, such as story, job, poll...
type Item struct {
	ID    int64
	Time  int64
	Score int
	URL   string
	Title string
}
