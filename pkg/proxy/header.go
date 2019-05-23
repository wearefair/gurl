package proxy

const (
	// This is the default proxy header for forwarding the request
	DefaultProxyTargetHeader = "x-proxy-target"
)

var (
	ProxyTargetHeader = DefaultProxyTargetHeader
)

// SetProxyTargetHeader sets the header key
func SetProxyTargetHeader(headerKey string) {
	ProxyTargetHeader = headerKey
}
