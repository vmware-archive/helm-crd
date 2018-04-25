package main

import (
	"testing"

	"github.com/arschles/assert"
)

// TODO: add more tests

func Test_resolveURL(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		chartURL  string
		wantedURL string
	}{
		{
			"absolute url",
			"http://www.google.com",
			"http://charts.example.com/repo/wordpress-0.1.0.tgz",
			"http://charts.example.com/repo/wordpress-0.1.0.tgz",
		},
		{
			"absolute url padding spaces",
			"http://www.google.com",
			" http://charts.example.com/repo/wordpress-0.1.0.tgz ",
			"http://charts.example.com/repo/wordpress-0.1.0.tgz",
		},
		{
			"relative url",
			"http://charts.example.com/repo",
			"wordpress-0.1.0.tgz",
			"http://charts.example.com/repo/wordpress-0.1.0.tgz",
		},
		{
			"relative url without a scheme in base url",
			"charts.example.com/repo",
			"wordpress-0.1.0.tgz",
			"charts.example.com/repo/wordpress-0.1.0.tgz",
		},
		{
			"relative url with padding spaces",
			" http://charts.example.com/repo ",
			" wordpress-0.1.0.tgz ",
			"http://charts.example.com/repo/wordpress-0.1.0.tgz",
		},
		{
			"relative url with index.yaml in base url",
			"http://charts.example.com/repo/index.yaml",
			"wordpress-0.1.0.tgz",
			"http://charts.example.com/repo/wordpress-0.1.0.tgz",
		},
		{
			"relative url with index.yaml in base url and padding spaces",
			" http://charts.example.com/repo/index.yaml ",
			" wordpress-0.1.0.tgz ",
			"http://charts.example.com/repo/wordpress-0.1.0.tgz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chartURL, _ := resolveURL(tt.baseURL, tt.chartURL)
			assert.Equal(t, chartURL, tt.wantedURL, "url")
		})
	}
}
