package util

import (
	"net"

	"github.com/wearefair/gurl/log"
)

// Use net/listen to pick a randomly available port for us to use
func GetAvailablePort() (string, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return "", log.WrapError(err)
	}
	defer listener.Close()

	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return "", log.WrapError(err)
	}
	return port, nil
}
