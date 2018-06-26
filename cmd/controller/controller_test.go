package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	helmCRDApi "github.com/bitnami-labs/helm-crd/pkg/apis/helm.bitnami.com/v1"
	helmCrdV1 "github.com/bitnami-labs/helm-crd/pkg/apis/helm.bitnami.com/v1"
	helmCRDFake "github.com/bitnami-labs/helm-crd/pkg/client/clientset/versioned/fake"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/repo"
)

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

// Fake server for repositories and charts
type fakeHTTPClient struct {
	repoURLs  []string
	chartURLs []string
	index     *repo.IndexFile
}

func (f *fakeHTTPClient) Do(h *http.Request) (*http.Response, error) {
	for _, repoURL := range f.repoURLs {
		if h.URL.String() == fmt.Sprintf("%sindex.yaml", repoURL) {
			// Return fake chart index (not customizable per repo)
			body, err := json.Marshal(*f.index)
			if err != nil {
				fmt.Printf("Error! %v", err)
			}
			return &http.Response{StatusCode: 200, Body: ioutil.NopCloser(bytes.NewReader(body))}, nil
		}
	}
	for _, chartURL := range f.chartURLs {
		if h.URL.String() == chartURL {
			// Fake chart response
			return &http.Response{StatusCode: 200, Body: ioutil.NopCloser(bytes.NewReader([]byte{}))}, nil
		}
	}
	// Unexpected path
	return &http.Response{StatusCode: 404}, fmt.Errorf("Unexpected path")
}

func fakeLoadChart(in io.Reader) (*chart.Chart, error) {
	return &chart.Chart{}, nil
}

func prepareTestController(hrs []helmCRDApi.HelmRelease, existingTillerReleases []string) *Controller {
	var repoURLs []string
	var chartURLs []string
	entries := map[string]repo.ChartVersions{}
	var hrObjects []runtime.Object
	for _, hr := range hrs {
		repoURLs = append(repoURLs, hr.Spec.RepoURL)
		chartMeta := chart.Metadata{Name: hr.Spec.ChartName, Version: hr.Spec.Version}
		chartURL := fmt.Sprintf("%s%s-%s.tgz", hr.Spec.RepoURL, hr.Spec.ChartName, hr.Spec.Version)
		chartURLs = append(chartURLs, chartURL)
		chartVersion := repo.ChartVersion{Metadata: &chartMeta, URLs: []string{chartURL}}
		chartVersions := []*repo.ChartVersion{&chartVersion}
		entries[hr.Spec.ChartName] = chartVersions
		hrObjects = append(hrObjects, &hr)
	}
	index := &repo.IndexFile{APIVersion: "v1", Generated: time.Now(), Entries: entries}
	netClient := fakeHTTPClient{repoURLs, chartURLs, index}
	helmClient := helm.FakeClient{}
	for _, r := range existingTillerReleases {
		helmClient.Rels = append(helmClient.Rels, &release.Release{Name: r})
	}
	clientset := helmCRDFake.NewSimpleClientset(hrObjects...)
	kubeClient := fake.NewSimpleClientset()
	controller := NewController(clientset, kubeClient, &helmClient, &netClient, fakeLoadChart)
	for _, hr := range hrs {
		controller.informer.GetIndexer().Add(&hr)
	}
	return controller
}

func TestHelmReleaseAdded(t *testing.T) {
	myNsFoo := metav1.ObjectMeta{
		Namespace: "myns",
		Name:      "foo",
	}
	h := helmCRDApi.HelmRelease{
		ObjectMeta: myNsFoo,
		Spec: helmCRDApi.HelmReleaseSpec{
			RepoURL:   "http://charts.example.com/repo/",
			ChartName: "foo",
			Version:   "v1.0.0",
		},
	}
	expectedRelease := fmt.Sprintf("%s-%s", myNsFoo.Namespace, myNsFoo.Name)
	controller := prepareTestController([]helmCRDApi.HelmRelease{h}, []string{})

	err := controller.updateRelease("myns/foo")
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	rels, err := controller.helmClient.ListReleases()
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if rels.Releases[0].Name != expectedRelease {
		t.Errorf("Expected release named %s received %s", expectedRelease, rels.Releases[0].Name)
	}
	if rels.Releases[0].Namespace != myNsFoo.Namespace {
		t.Errorf("Expected release in namespace %s received %s", myNsFoo.Namespace, rels.Releases[0].Namespace)
	}
	// We cannot check that the rest of the chart properties as properly set
	// because the fake InstallReleaseFromChart ignores the given chart
}

func TestHelmReleaseAddedWithReleaseName(t *testing.T) {
	myNsFoo := metav1.ObjectMeta{
		Namespace: "myns",
		Name:      "foo",
	}
	h := helmCRDApi.HelmRelease{
		ObjectMeta: myNsFoo,
		Spec: helmCRDApi.HelmReleaseSpec{
			ReleaseName: "not-foo",
			RepoURL:     "http://charts.example.com/repo/",
			ChartName:   "foo",
			Version:     "v1.0.0",
		},
	}
	controller := prepareTestController([]helmCRDApi.HelmRelease{h}, []string{})

	err := controller.updateRelease("myns/foo")
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	rels, err := controller.helmClient.ListReleases()
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if rels.Releases[0].Name != h.Spec.ReleaseName {
		t.Errorf("Expected release named %s received %s", h.Spec.ReleaseName, rels.Releases[0].Name)
	}
}

func TestHelmReleaseUpdated(t *testing.T) {
	releaseName := "bar"
	myNsFoo := metav1.ObjectMeta{
		Namespace: "myns",
		Name:      "foo",
	}
	h := helmCRDApi.HelmRelease{
		ObjectMeta: myNsFoo,
		Spec: helmCRDApi.HelmReleaseSpec{
			ReleaseName: releaseName,
			RepoURL:     "http://charts.example.com/repo/",
			ChartName:   "foo",
			Version:     "v1.0.0",
		},
	}
	controller := prepareTestController([]helmCRDApi.HelmRelease{h}, []string{releaseName})

	err := controller.updateRelease("myns/foo")
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	rels, err := controller.helmClient.ListReleases()
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	// We cannot test that the release content changes because fake UpdateReleaseResponse
	// does not modify the release
	if len(rels.Releases) != 1 {
		t.Errorf("Unexpected amount of releases %d, it should update the existing one", len(rels.Releases))
	}
}

func TestHelmReleaseDeleted(t *testing.T) {
	releaseName := "bar"
	myNsFoo := metav1.ObjectMeta{
		Namespace:         "myns",
		Name:              "foo",
		DeletionTimestamp: &metav1.Time{},
		Finalizers:        []string{releaseFinalizer},
	}
	h := helmCRDApi.HelmRelease{
		ObjectMeta: myNsFoo,
		Spec: helmCRDApi.HelmReleaseSpec{
			ReleaseName: releaseName,
			RepoURL:     "http://charts.example.com/repo/",
			ChartName:   "foo",
			Version:     "v1.0.0",
		},
	}
	controller := prepareTestController([]helmCRDApi.HelmRelease{h}, []string{releaseName})

	err := controller.updateRelease("myns/foo")
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	rels, err := controller.helmClient.ListReleases()
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if len(rels.Releases) != 0 {
		t.Errorf("Unexpected amount of releases %d, it should be empty", len(rels.Releases))
	}
}
