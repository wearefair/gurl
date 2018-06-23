package log

import "github.com/golang/glog"

// WrapError is a helper func to log the error passed in and also return the err
func WrapError(level glog.Level, err error) error {
	if err != nil && glog.V(level) {
		glog.ErrorDepth(1, err)
	}
	return err
}
