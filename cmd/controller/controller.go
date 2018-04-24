package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/repo"

	helmCrdV1 "github.com/bitnami-labs/helm-crd/pkg/apis/helm.bitnami.com/v1"
	helmClientset "github.com/bitnami-labs/helm-crd/pkg/client/clientset/versioned"
)

const (
	defaultNamespace      = metav1.NamespaceSystem
	defaultRepoURL        = "https://kubernetes-charts.storage.googleapis.com"
	defaultTimeoutSeconds = 180
	maxRetries            = 5
)

// Controller is a cache.Controller for acting on Helm CRD objects
type Controller struct {
	queue      workqueue.RateLimitingInterface
	informer   cache.SharedIndexInformer
	kubeClient kubernetes.Interface
	helmClient *helm.Client
}

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// NewController creates a Controller
func NewController(clientset helmClientset.Interface, kubeClient kubernetes.Interface) cache.Controller {
	lw := cache.NewListWatchFromClient(clientset.HelmV1().RESTClient(), "helmreleases", metav1.NamespaceAll, fields.Everything())

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	informer := cache.NewSharedIndexInformer(
		lw,
		&helmCrdV1.HelmRelease{},
		0, // No periodic resync
		cache.Indexers{},
	)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			if err == nil {
				queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
	})

	log.Printf("Using tiller host: %s", settings.TillerHost)

	return &Controller{
		informer:   informer,
		queue:      queue,
		kubeClient: kubeClient,
		helmClient: helm.NewClient(helm.Host(settings.TillerHost)),
	}
}

// HasSynced returns true once this controller has completed an
// initial resource listing
func (c *Controller) HasSynced() bool {
	return c.informer.HasSynced()
}

// LastSyncResourceVersion is the resource version observed when last
// synced with the underlying store. The value returned is not
// synchronized with access to the underlying store and is not
// thread-safe.
func (c *Controller) LastSyncResourceVersion() string {
	return c.informer.LastSyncResourceVersion()
}

// Run begins processing items, and will continue until a value is
// sent down stopCh.  It's an error to call Run more than once.  Run
// blocks; call via go.
func (c *Controller) Run(stopCh <-chan struct{}) {
	log.Print("Starting HelmReleases controller")

	defer utilruntime.HandleCrash()

	defer c.queue.ShutDown()

	go c.informer.Run(stopCh)

	// Set up a helm home dir sufficient to fool the rest of helm
	// client code
	os.MkdirAll(settings.Home.Archive(), 0755)
	os.MkdirAll(settings.Home.Repository(), 0755)
	ioutil.WriteFile(settings.Home.RepositoryFile(),
		[]byte("apiVersion: v1\nrepositories: []"), 0644)

	if !cache.WaitForCacheSync(stopCh, c.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}
	log.Print("Cache synchronised, starting main loop")

	wait.Until(c.runWorker, time.Second, stopCh)

	log.Print("Shutting down controller")
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
		// continue looping
	}
}

func (c *Controller) processNextItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}

	defer c.queue.Done(key)
	err := c.updateRelease(key.(string))
	if err == nil {
		// No error, reset the ratelimit counters
		c.queue.Forget(key)
	} else if c.queue.NumRequeues(key) < maxRetries {
		log.Printf("Error updating %s, will retry: %v", key, err)
		c.queue.AddRateLimited(key)
	} else {
		// err != nil and too many retries
		log.Printf("Error updating %s, giving up: %v", key, err)
		c.queue.Forget(key)
		utilruntime.HandleError(err)
	}

	return true
}

func fetchUrl(netClient httpClient, reqURL, authHeader string) (*http.Response, error) {
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	if len(authHeader) > 0 {
		req.Header.Set("Authorization", authHeader)
	}
	return netClient.Do(req)
}

func fetchRepoIndex(repoURL string, authHeader string) (*repo.IndexFile, error) {

	parsedURL, err := url.ParseRequestURI(strings.TrimSpace(repoURL))
	if err != nil {
		return nil, err
	}
	parsedURL.Path = path.Join(parsedURL.Path, "index.yaml")

	netClient := &http.Client{
		Timeout: time.Second * defaultTimeoutSeconds,
	}

	res, err := fetchUrl(netClient, parsedURL.String(), authHeader)
	if res != nil {
		defer res.Body.Close()
	}
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.New("repo index request failed")
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	index := &repo.IndexFile{}
	err = yaml.Unmarshal(body, index)
	if err != nil {
		return index, err
	}
	index.SortEntries()
	return index, nil
}

func findChartInRepoIndex(repoIndex *repo.IndexFile, chartName, chartVersion string) (string, error) {
	errMsg := fmt.Sprintf("chart %q", chartName)
	if chartVersion != "" {
		errMsg = fmt.Sprintf("%s version %q", errMsg, chartVersion)
	}
	cv, err := repoIndex.Get(chartName, chartVersion)
	if err != nil {
		return "", fmt.Errorf("%s not found in repository", errMsg)
	}

	if len(cv.URLs) == 0 {
		return "", fmt.Errorf("%s has no downloadable URLs", errMsg)
	}
	return cv.URLs[0], nil
}

func fetchChart(chartURL, authHeader string) (*chart.Chart, error) {
	netClient := &http.Client{
		Timeout: time.Second * defaultTimeoutSeconds,
	}

	res, err := fetchUrl(netClient, chartURL, authHeader)
	if res != nil {
		defer res.Body.Close()
	}
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.New("chart download request failed")
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return chartutil.LoadArchive(bytes.NewReader(body))
}

func releaseName(ns, name string) string {
	return fmt.Sprintf("%s-%s", ns, name)
}

func isNotFound(err error) bool {
	// Ideally this would be `grpc.Code(err) == codes.NotFound`,
	// but it seems helm doesn't return grpc codes
	return strings.Contains(grpc.ErrorDesc(err), "not found")
}

func resolveURL(base, ref string) (string, error) {
	refURL, err := url.Parse(ref)
	if err != nil {
		baseURL, err := url.Parse(base)
		if err != nil {
			return "", err
		}
		baseURL.Path = path.Join(baseURL.Path, ref)
		return baseURL.String(), nil
	}
	return refURL.String(), nil
}

func (c *Controller) updateRelease(key string) error {
	obj, exists, err := c.informer.GetIndexer().GetByKey(key)
	if err != nil {
		return fmt.Errorf("error fetching object with key %s from store: %v", key, err)
	}

	if !exists {
		log.Printf("HelmRelease %s has gone, uninstalling chart", key)
		ns, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return err
		}
		_, err = c.helmClient.DeleteRelease(
			releaseName(ns, name),
			helm.DeletePurge(true),
		)
		if err != nil {
			// fixme: ignore "not found" or similar
			return err
		}
		return nil
	}

	helmObj := obj.(*helmCrdV1.HelmRelease)

	repoURL := helmObj.Spec.RepoURL
	if repoURL == "" {
		// FIXME: Make configurable
		repoURL = defaultRepoURL
	}

	authHeader := ""
	if helmObj.Spec.Auth.Header != nil {
		namespace := os.Getenv("POD_NAMESPACE")
		if namespace == "" {
			namespace = defaultNamespace
		}

		secret, err := c.kubeClient.Core().Secrets(namespace).Get(helmObj.Spec.Auth.Header.SecretKeyRef.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		authHeader = string(secret.Data[helmObj.Spec.Auth.Header.SecretKeyRef.Key])
	}

	log.Printf("Downloading repo %s index...", repoURL)
	repoIndex, err := fetchRepoIndex(repoURL, authHeader)
	if err != nil {
		return err
	}

	chartURL, err := findChartInRepoIndex(repoIndex, helmObj.Spec.ChartName, helmObj.Spec.Version)
	if err != nil {
		return err
	}

	chartURL, err = resolveURL(repoURL, chartURL)
	if err != nil {
		return err
	}

	log.Printf("Downloading %s ...", chartURL)
	chartRequested, err := fetchChart(chartURL, authHeader)
	if err != nil {
		return err
	}

	rlsName := releaseName(helmObj.Namespace, helmObj.Name)

	var rel *release.Release

	_, err = c.helmClient.ReleaseHistory(rlsName, helm.WithMaxHistory(1))
	if err != nil {
		if !isNotFound(err) {
			return err
		}
		log.Printf("Installing release %s into namespace %s", rlsName, helmObj.Namespace)
		res, err := c.helmClient.InstallReleaseFromChart(
			chartRequested,
			helmObj.Namespace,
			helm.ValueOverrides([]byte(helmObj.Spec.Values)),
			helm.ReleaseName(rlsName),
		)
		if err != nil {
			return err
		}
		rel = res.GetRelease()
	} else {
		log.Printf("Updating release %s", rlsName)
		res, err := c.helmClient.UpdateReleaseFromChart(
			rlsName,
			chartRequested,
			helm.UpdateValueOverrides([]byte(helmObj.Spec.Values)),
			//helm.UpgradeForce(true), ?
		)
		if err != nil {
			return err
		}
		rel = res.GetRelease()
	}

	status, err := c.helmClient.ReleaseStatus(rel.Name)
	if err == nil {
		log.Printf("Installed/updated release %s (status %s)", rel.Name, status.Info.Status.Code)
	} else {
		log.Printf("Unable to fetch release status for %s: %v", rel.Name, err)
	}

	return nil
}
