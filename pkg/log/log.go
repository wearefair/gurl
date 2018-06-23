// Thin wrappers around the glog package to faciliate the most common case of logging on level 1.
//
// These are less performant than using glog directly as we can't short-circut any work
// if we aren't at or above level 1.
// However because of the use-case of this program (CLI tool to debug gRPC apis), we consider the
// performance pentalty to be negligible.
package log

import "github.com/golang/glog"

const level = 1

func Error(args ...interface{}) {
	if glog.V(level) {
		glog.Error(args...)
	}
}

func Errorf(format string, args ...interface{}) {
	if glog.V(level) {
		glog.Errorf(format, args...)
	}
}

func Info(args ...interface{}) {
	if glog.V(level) {
		glog.Info(args...)
	}
}

func Infof(format string, args ...interface{}) {
	if glog.V(level) {
		glog.Infof(format, args...)
	}
}

func Warning(args ...interface{}) {
	if glog.V(level) {
		glog.Warning(args...)
	}
}

func Warningf(format string, args ...interface{}) {
	if glog.V(level) {
		glog.Warningf(format, args...)
	}
}

// LogError is a helper func to log the error passed in and also return the err.
// This helps with patterns where you want to log the error before returning from a function.
//  if err != nil {
//    return nil, log.LogError(err)
//  }
func LogError(err error) error {
	if err != nil && glog.V(level) {
		glog.ErrorDepth(1, err)
	}
	return err
}
