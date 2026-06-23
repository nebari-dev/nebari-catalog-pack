// Package argocd nudges ArgoCD to reconcile after a GitOps commit and reports
// Application status. It mirrors the add-software-pack action's wait-for-app
// flow: annotate the root app-of-apps with a refresh request, then poll the
// child Application until it exists and becomes Healthy/Synced.
//
// ArgoCD is never the thing that writes to git — the gitops package does that.
// This package only observes and pokes the in-cluster ArgoCD.
package argocd

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// applicationGVR identifies ArgoCD Applications.
var applicationGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "applications",
}

// Status is a compact view of an Application's reconciliation state.
type Status struct {
	Exists bool
	Health string // Healthy, Progressing, Degraded, Missing, ...
	Sync   string // Synced, OutOfSync, ...
}

// Ready reports the app reached a steady, healthy, synced state.
func (s Status) Ready() bool {
	return s.Exists && s.Health == "Healthy" && s.Sync == "Synced"
}

// Client talks to ArgoCD's Applications in a namespace.
type Client struct {
	dyn       dynamic.Interface
	namespace string
	rootApp   string
}

// New builds a Client using in-cluster config, falling back to the default
// kubeconfig loading rules (KUBECONFIG / ~/.kube/config) for local use.
func New(namespace, rootApp string) (*Client, error) {
	cfg, err := restConfig()
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{dyn: dyn, namespace: namespace, rootApp: rootApp}, nil
}

func restConfig() (*rest.Config, error) {
	if c, err := rest.InClusterConfig(); err == nil {
		return c, nil
	}
	loading := clientcmd.NewDefaultClientConfigLoadingRules()
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loading, &clientcmd.ConfigOverrides{}).ClientConfig()
}

// Refresh annotates the root app-of-apps to trigger an immediate reconcile,
// so a freshly committed Application is created without waiting for the next
// polling cycle. Idempotent — ArgoCD strips the annotation on sync.
func (c *Client) Refresh(ctx context.Context) error {
	patch := []byte(`{"metadata":{"annotations":{"argocd.argoproj.io/refresh":"normal"}}}`)
	_, err := c.dyn.Resource(applicationGVR).Namespace(c.namespace).
		Patch(ctx, c.rootApp, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("refresh %s/%s: %w", c.namespace, c.rootApp, err)
	}
	return nil
}

// Get returns the current status of an Application.
func (c *Client) Get(ctx context.Context, name string) (Status, error) {
	obj, err := c.dyn.Resource(applicationGVR).Namespace(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		// Not-found is a normal "doesn't exist yet" state, not an error here.
		return Status{Exists: false}, nil
	}
	health, _, _ := nestedString(obj.Object, "status", "health", "status")
	sync, _, _ := nestedString(obj.Object, "status", "sync", "status")
	return Status{Exists: true, Health: health, Sync: sync}, nil
}

// List returns the status of every Application in the namespace, keyed by
// Application name. Used to surface which packs are already installed.
func (c *Client) List(ctx context.Context) (map[string]Status, error) {
	list, err := c.dyn.Resource(applicationGVR).Namespace(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list applications in %s: %w", c.namespace, err)
	}
	out := make(map[string]Status, len(list.Items))
	for i := range list.Items {
		obj := list.Items[i].Object
		name := list.Items[i].GetName()
		health, _, _ := nestedString(obj, "status", "health", "status")
		sync, _, _ := nestedString(obj, "status", "sync", "status")
		out[name] = Status{Exists: true, Health: health, Sync: sync}
	}
	return out, nil
}

// RootApp returns the configured app-of-apps name (for status display).
func (c *Client) RootApp() string { return c.rootApp }

// WaitReady polls until the named Application is Healthy+Synced or the timeout
// elapses. It returns the last observed status.
func (c *Client) WaitReady(ctx context.Context, name string, timeout, interval time.Duration) (Status, error) {
	deadline := time.Now().Add(timeout)
	var last Status
	for {
		st, err := c.Get(ctx, name)
		if err != nil {
			return last, err
		}
		last = st
		if st.Ready() {
			return st, nil
		}
		if time.Now().After(deadline) {
			return last, fmt.Errorf("timed out waiting for %s (health=%q sync=%q)", name, last.Health, last.Sync)
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// nestedString is a tiny stand-in for unstructured.NestedString to avoid the
// extra import; fields are always strings in ArgoCD status.
func nestedString(obj map[string]any, fields ...string) (string, bool, error) {
	cur := any(obj)
	for _, f := range fields {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", false, nil
		}
		cur, ok = m[f]
		if !ok {
			return "", false, nil
		}
	}
	s, ok := cur.(string)
	return s, ok, nil
}
