// Thin wrappers around the glog package. Level is the minimum log level that'll be logged.
// In glog, the lower the verbose level, the more messages will be logged.
//
// Example, if level = 1, then anything of level 1 will log, but anything of 2 will not.
// This is an inversion of the severity level. Example - INFO is 0 and ERROR is 2.
//
// Therefore, we'll set things to be as follows for verbosity...
// 2 - INFO
// 1 - WARN
// 0 - ERROR
//
package log

import "github.com/golang/glog"

const (
	errorLevel = iota
	warnLevel
	infoLevel
)

func Error(args ...interface{}) {
	if glog.V(errorLevel) {
		glog.Error(args...)
	}
}

func Errorf(format string, args ...interface{}) {
	if glog.V(errorLevel) {
		glog.Errorf(format, args...)
	}
}

func Info(args ...interface{}) {
	if glog.V(infoLevel) {
		glog.Info(args...)
	}
}

func Infof(format string, args ...interface{}) {
	if glog.V(infoLevel) {
		glog.Infof(format, args...)
	}
}

func Warning(args ...interface{}) {
	if glog.V(warnLevel) {
		glog.Warning(args...)
	}
}

func Warningf(format string, args ...interface{}) {
	if glog.V(warnLevel) {
		glog.Warningf(format, args...)
	}
}

// LogAndReturn is a helper func to log the error passed in and also return the err.
// This helps with patterns where you want to log the error before returning from a function.
//  if err != nil {
//    return nil, log.LogAndReturn(err)
//  }
func LogAndReturn(err error) error {
	if err != nil && glog.V(errorLevel) {
		glog.ErrorDepth(1, err)
	}
	return err
}
