package util

import (
	"errors"
	"regexp"

	"github.com/wearefair/gurl/log"
)

const (
	// protocol://context/service:port/grpc/service-method
	uriRegex = `((?P<protocol>[a-z0-9]{2,4})(?:\:\/\/)((?P<context>[0-9a-z._-]+)(?:\/))?)?(?P<service>[0-9a-z-_.]+)(?:\:)(?P<port>[0-9]{2,5})(?:\/)(?P<rpc>[0-9a-zA-Z._-]+)(?:\/)(?P<method>[0-9a-zA-Z._-]+)`

	K8Protocol = "k8"
)

var (
	errInvalidURIFormat = errors.New("URI must be in the form of host:port/service/method")
	uriRegexp           = regexp.MustCompile(uriRegex)
)

// Represents a deconstructed uri structure
type URI struct {
	Protocol string
	Context  string
	Service  string
	Port     string
	RPC      string
	Method   string
}

// ParseURI parses URI and returns it in an expected format
func ParseURI(uri string) (*URI, error) {
	namedMatches := make(map[string]string)
	// host:port/service/method
	submatches := uriRegexp.FindStringSubmatch(uri)
	if submatches == nil {
		log.Logger().Error(errInvalidURIFormat.Error())
		return nil, errInvalidURIFormat
	}
	for i, name := range uriRegexp.SubexpNames() {
		// The first index is going to be the whole match
		if i == 0 || name == "" {
			continue
		}
		namedMatches[name] = submatches[i]
	}
	if len(namedMatches) < 1 {
		log.Logger().Error(errInvalidURIFormat.Error())
		return nil, errInvalidURIFormat
	}
	uriWrapper := &URI{
		Protocol: namedMatches["protocol"],
		Context:  namedMatches["context"],
		Service:  namedMatches["service"],
		Port:     namedMatches["port"],
		RPC:      namedMatches["rpc"],
		Method:   namedMatches["method"],
	}
	return uriWrapper, nil
}
