package k8

import "k8s.io/client-go/tools/clientcmd"

// K8Config is just a helper function for getting the default K8 configs from
// $HOME/.kube/config
func DefaultConfig() clientcmd.ClientConfig {
	// if you want to change the loading rules (which files in which order), you can do so here
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	// if you want to change override values or bind them to flags, there are methods to help you
	configOverrides := &clientcmd.ConfigOverrides{}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
}
