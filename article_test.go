package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchArticle(t *testing.T) {
	paragraph := strings.Repeat("Readable article content with enough detail. ", 20)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>Example</title><meta name="author" content="Ada"></head><body><article><h1>Example</h1><p>` + paragraph + `</p></article></body></html>`))
	}))
	defer server.Close()

	content, author, err := fetchArticle(context.Background(), server.Client(), server.URL)
	if err != nil {
		t.Fatalf("fetch article: %v", err)
	}
	if !strings.Contains(content, "Readable article content") {
		t.Fatalf("readable content was not extracted: %q", content)
	}
	if author != "Ada" {
		t.Fatalf("unexpected author: %q", author)
	}
}

func TestFetchArticleRejectsNonHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("not a PDF"))
	}))
	defer server.Close()

	if _, _, err := fetchArticle(context.Background(), server.Client(), server.URL); err == nil {
		t.Fatal("expected non-HTML content to be rejected")
	}
}
