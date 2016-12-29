package martini

import (
	"log"
	"net/http"
	"time"
)

// Logger returns a middleware handler that logs the request as it goes in and the response as it goes out.
func Logger() Handler {
	return func(res http.ResponseWriter, req *http.Request, c Context, log *log.Logger) {
		start := time.Now()

		Ip = req.Header.Get("X-Slb-Forwarded-For")
		if Ip == "" {
			Ip = req.Header.Get("X-Nginx-Forwarded-For")
			if Ip == "" {
				Ip = req.Header.Get("X-Forwarded-For")
				if Ip == "" {
					Ip = req.RemoteAddr
				}
			}
		}

		requestId := req.Header.Get("request-id")

		log.Printf("Started %s %s for %s %s", req.Method, req.URL.Path, Ip, requestId)

		rw := res.(ResponseWriter)
		c.Next()

		log.Printf("Completed %v %s %s %s %s %s in %v\n", rw.Status(), http.StatusText(rw.Status()), requestId, req.Method, req.URL.Path, Ip, time.Since(start).Seconds())
	}
}
