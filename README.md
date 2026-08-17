# HN-feed

A customized Hacker News Atom feed. The service checks the top 50 stories every 20 minutes, keeps stories with a score of at least 50, and extracts readable article content when available.

Each Atom entry uses its Hacker News item URL as its stable ID. External stories still link to the original article; Hacker News self-posts link to their discussion page.

## Run locally

Requires Go 1.26.6 or later in the Go 1.26 release line.

```sh
go run .
```

The service writes `hn-feed.atom` and `last_updated.txt` to the current directory. Set `HN_FEED_OUTPUT_DIR` to write them elsewhere:

```sh
HN_FEED_OUTPUT_DIR=/tmp/hackernews-feed go run .
```

## Test

```sh
go test ./...
go vet ./...
govulncheck ./...
```

## Docker

```sh
docker compose up -d
```

The included Compose file writes generated files to `/tmp/hackernews-feed` on the host.
