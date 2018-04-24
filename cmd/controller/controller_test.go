package main

import (
	"testing"

	"github.com/arschles/assert"
)

// TODO: add more tests

func Test_resolveURL(t *testing.T) {
	baseURL := "http://charts.example.com"
	tests := []struct {
		name      string
		chartURL  string
		wantedURL string
	}{
		{"absolute url", "http://charts.example.com/wordpress-0.1.0.tgz", "http://charts.example.com/wordpress-0.1.0.tgz"},
		{"absolute url with leading spaces", " http://charts.example.com/wordpress-0.1.0.tgz", "http://charts.example.com/wordpress-0.1.0.tgz"},
		{"absolute url with trailing spaces", "http://charts.example.com/wordpress-0.1.0.tgz ", "http://charts.example.com/wordpress-0.1.0.tgz"},
		{"relative url", "wordpress-0.1.0.tgz", "http://charts.example.com/wordpress-0.1.0.tgz"},
		{"relative url with leading spaces", " wordpress-0.1.0.tgz", "http://charts.example.com/wordpress-0.1.0.tgz"},
		{"relative url with trailing spaces", "wordpress-0.1.0.tgz ", "http://charts.example.com/wordpress-0.1.0.tgz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chartURL, _ := resolveURL(baseURL, tt.chartURL)
			assert.Equal(t, chartURL, tt.wantedURL, "url")
		})
	}
}
