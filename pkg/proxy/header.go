package proxy

const (
	// This is the default proxy header for forwarding the request
	DefaultProxyTargetHeader = "x-gurl-proxy-target"
)

var (
	ProxyTargetHeader = DefaultProxyTargetHeader
)
