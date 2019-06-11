package log

import (
	"net/http"

	"github.com/wearefair/gurl/pkg/log"
)

// Middleware is logging middleware that logs the request/response info
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWrapper{ResponseWriter: w}
		next.ServeHTTP(rw, r)
		log.Infof("%s %s -%d", r.Method, r.URL, rw.status)
	})
}

// responseWrapper allows us to grab the info about a response and log it
type responseWrapper struct {
	http.ResponseWriter
	status int
}

func (o *responseWrapper) Header() http.Header {
	return o.ResponseWriter.Header()
}

func (o *responseWrapper) Write(b []byte) (int, error) {
	return o.ResponseWriter.Write(b)
}

func (o *responseWrapper) WriteHeader(code int) {
	o.status = code
	o.ResponseWriter.WriteHeader(code)
}
