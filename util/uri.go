package util

import (
	"errors"
	"regexp"

	"github.com/wearefair/gurl/log"
)

const (
	// Regexp that matches the expected uri structure - allows for -_. as special characters
	uriRegex = `([a-z]+)(?:\:)([0-9]{5})(?:\/)([0-9a-zA-Z._-]+)(?:\/)([0-9a-zA-Z._-]+)`
)

var (
	uriRegexp = regexp.MustCompile(uriRegex)
)

// Represents a deconstructed uri structure
type URI struct {
	Host    string
	Port    string
	Service string
	Method  string
}

func ParseURI(uri string) (*URI, error) {
	uriWrapper := &URI{}
	// host:port/service/method
	matches := uriRegexp.FindAllStringSubmatch(uri, -1)[0]
	if len(matches) < 5 {
		err := errors.New("URI must be in the form of host:port/service/method")
		log.Logger().Error(err.Error())
		return nil, err
	}
	// matches[0] is the entire string
	uriWrapper.Host = matches[1]
	uriWrapper.Port = matches[2]
	uriWrapper.Service = matches[3]
	uriWrapper.Method = matches[4]
	return uriWrapper, nil
}
