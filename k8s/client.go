package k8s

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

type Client struct {
	kubernetes.Interface
}

func New() (*Client, error) {
	_, clientset, err := GetClient()
	if err != nil {
		return nil, err
	}

	return &Client{
		clientset,
	}, nil
}

func GetClient() (*rest.Config, kubernetes.Interface, error) {
	var config *rest.Config
	var err error

	// Try to use in-cluster config
	if config, err = rest.InClusterConfig(); err != nil {
		slog.Debug("We are not in cluster - is this a local environment?")
		// If in-cluster config fails, fallback to KUBECONFIG or default kubeconfig file
		kubeconfigPath := ""
		if os.Getenv("KUBECONFIG") != "" {
			slog.Debug("Found KUBECONFIG environment variable", "KUBECONFIG", os.Getenv("KUBECONFIG"))
			kubeconfigPath = os.Getenv("KUBECONFIG")
		} else if home := homedir.HomeDir(); home != "" {
			slog.Debug("Falling back to user home", "HOME", home)
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		}

		if kubeconfigPath == "" {
			return nil, nil, errors.New("Cannot find KUBECONFIG or default kubeconfig file")
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, nil, err
		}
	}

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return config, nil, err
	}

	return config, clientset, nil
}
