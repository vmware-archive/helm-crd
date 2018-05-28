package main

import (
	"testing"

	"github.com/arschles/assert"
	helmCrdV1 "github.com/bitnami-labs/helm-crd/pkg/apis/helm.bitnami.com/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO: add more tests

func Test_resolveChartURL(t *testing.T) {
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
			"relative, repo url",
			"http://charts.example.com/repo/",
			"wordpress-0.1.0.tgz",
			"http://charts.example.com/repo/wordpress-0.1.0.tgz",
		},
		{
			"relative, repo index url",
			"http://charts.example.com/repo/index.yaml",
			"wordpress-0.1.0.tgz",
			"http://charts.example.com/repo/wordpress-0.1.0.tgz",
		},
		{
			"relative, repo url - no trailing slash",
			"http://charts.example.com/repo",
			"wordpress-0.1.0.tgz",
			"http://charts.example.com/wordpress-0.1.0.tgz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chartURL, err := resolveChartURL(tt.baseURL, tt.chartURL)
			assert.NoErr(t, err)
			assert.Equal(t, chartURL, tt.wantedURL, "url")
		})
	}
}

func TestAddFinalizer(t *testing.T) {
	tests := []struct {
		src               *helmCrdV1.HelmRelease
		expectedFinalizer []string
	}{
		{&helmCrdV1.HelmRelease{}, []string{"helm.bitnami.com/helmrelease"}},
		{&helmCrdV1.HelmRelease{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{"foo"}}}, []string{"foo", "helm.bitnami.com/helmrelease"}},
	}
	for _, tt := range tests {
		res := addFinalizer(tt.src)
		if !apiequality.Semantic.DeepEqual(res.ObjectMeta.Finalizers, tt.expectedFinalizer) {
			t.Errorf("Expecting %v received %v", tt.expectedFinalizer, res.ObjectMeta.Finalizers)
		}
	}
}

func TestRemoveFinalizer(t *testing.T) {
	tests := []struct {
		src               *helmCrdV1.HelmRelease
		expectedFinalizer []string
	}{
		{&helmCrdV1.HelmRelease{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{"helm.bitnami.com/helmrelease"}}}, []string{}},
		{&helmCrdV1.HelmRelease{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{"foo", "helm.bitnami.com/helmrelease", "bar"}}}, []string{"foo", "bar"}},
	}
	for _, tt := range tests {
		res := removeFinalizer(tt.src)
		if !apiequality.Semantic.DeepEqual(res.ObjectMeta.Finalizers, tt.expectedFinalizer) {
			t.Errorf("Expecting %v received %v", tt.expectedFinalizer, res.ObjectMeta.Finalizers)
		}
	}
}
