package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	readability "codeberg.org/readeck/go-readability/v2"
)

const maxArticleBytes = 10 << 20

func fetchArticle(ctx context.Context, client *http.Client, rawURL string) (content string, author string, err error) {
	pageURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("parse article URL: %w", err)
	}
	if (pageURL.Scheme != "http" && pageURL.Scheme != "https") || pageURL.Host == "" {
		return "", "", fmt.Errorf("unsupported article URL: %s", rawURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("create article request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetch article: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", "", fmt.Errorf("fetch article: unexpected status %s", resp.Status)
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil {
			return "", "", fmt.Errorf("parse article content type: %w", err)
		}
		if mediaType != "text/html" && mediaType != "application/xhtml+xml" {
			return "", "", fmt.Errorf("article is not HTML: %s", mediaType)
		}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxArticleBytes+1))
	if err != nil {
		return "", "", fmt.Errorf("read article: %w", err)
	}
	if len(body) > maxArticleBytes {
		return "", "", fmt.Errorf("article exceeds %d bytes", maxArticleBytes)
	}

	article, err := readability.FromReader(bytes.NewReader(body), pageURL)
	if err != nil {
		return "", "", fmt.Errorf("extract readable article: %w", err)
	}
	var rendered bytes.Buffer
	if err := article.RenderHTML(&rendered); err != nil {
		return "", "", fmt.Errorf("render readable article: %w", err)
	}
	return rendered.String(), strings.TrimSpace(article.Byline()), nil
}
