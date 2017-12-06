package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/repo"

	helmCrdV1 "github.com/bitnami/helm-crd/pkg/apis/helm.bitnami.com/v1"
	helmClientset "github.com/bitnami/helm-crd/pkg/client/clientset/versioned"
)

const (
	defaultRepoURL = "https://kubernetes-charts.storage.googleapis.com"
	maxRetries     = 5
)

// Controller is a cache.Controller for acting on Helm CRD objects
type Controller struct {
	queue      workqueue.RateLimitingInterface
	informer   cache.SharedIndexInformer
	helmClient *helm.Client
}

// NewController creates a Controller
func NewController(clientset helmClientset.Interface) cache.Controller {
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

func releaseName(ns, name string) string {
	return fmt.Sprintf("%s-%s", ns, name)
}

func isNotFound(err error) bool {
	// Ideally this would be `grpc.Code(err) == codes.NotFound`,
	// but it seems helm doesn't return grpc codes
	return strings.Contains(grpc.ErrorDesc(err), "not found")
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

	// FIXME: make configurable
	keyring := "/keyring/pubring.gpg"

	dl := downloader.ChartDownloader{
		HelmHome: settings.Home,
		Out:      os.Stdout,
		Keyring:  keyring,
		Getters:  getter.All(settings),
		Verify:   downloader.VerifyNever, // FIXME
	}

	repoURL := helmObj.Spec.RepoURL
	if repoURL == "" {
		// FIXME: Make configurable
		repoURL = defaultRepoURL
	}

	certFile := ""
	keyFile := ""
	caFile := ""
	chartURL, err := repo.FindChartInRepoURL(repoURL, helmObj.Spec.ChartName, helmObj.Spec.Version, certFile, keyFile, caFile, getter.All(settings))
	if err != nil {
		return err
	}

	log.Printf("Downloading %s ...", chartURL)
	fname, _, err := dl.DownloadTo(chartURL, helmObj.Spec.Version, settings.Home.Archive())
	if err != nil {
		return err
	}
	log.Printf("Downloaded %s to %s", chartURL, fname)
	chartRequested, err := chartutil.LoadFile(fname) // fixme: just download to ram buf
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
