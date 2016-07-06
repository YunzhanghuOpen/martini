package martini

import (
    "fmt"
    "log"
    "net/http"
    "time"
    "reflect"

    "redpacket"
    "github.com/influxdata/influxdb/client/v2"
)

// Logger returns a middleware handler that logs the request as it goes in and the response as it goes out.
func InfluxLogger() Handler {
    return func(res http.ResponseWriter, req *http.Request, c Context, log *log.Logger, msgQ chan interface{}) {
        start := time.Now()

        addr := req.Header.Get("X-Real-IP")
        if addr == "" {
            addr = req.Header.Get("X-Forwarded-For")
            if addr == "" {
                addr = req.RemoteAddr
            }
        }

        log.Printf("Started %s %s for %s", req.Method, req.URL.Path, addr)

        rw := res.(ResponseWriter)
        c.Next()

        latency := time.Since(start)
        log.Printf("Completed %v %s in %v\n", rw.Status(), http.StatusText(rw.Status()), latency)

        // TODO(yiyang): test concurrent
        val := c.Get(reflect.TypeOf(&redpacket.Token{}))
        var token *redpacket.Token
        if val.IsValid() {
            token = val.Interface().(*redpacket.Token)
        } else {
            token = nil
        }

        go func() {
            var tags map[string]string
            var fields map[string]interface{}
            if token != nil {
                tags = map[string]string{
                    "bind_uid":  fmt.Sprintf("%d", token.Base.BindUID),
                    "rp_uid":    fmt.Sprintf("%d", token.Ext.RedpacketUID),
                    "real_uid":  fmt.Sprintf("%d", token.Ext.RealUID),
                    "dealer_id": fmt.Sprintf("%d", token.Base.DealerID),
                    "req":       req.URL.Path,
                    "ip":        addr,
                }

                fields = map[string]interface{}{
                    "dealer_username": token.Ext.DealerUsername,
                    "dealer_code":     token.Ext.DealerCode,
                    "status":          rw.Status(),
                    "latency":         latency,
                }
            } else {
                tags = map[string]string{
                    "req":       req.URL.Path,
                    "ip":        addr,
                }

                fields = map[string]interface{}{
                    "status":          rw.Status(),
                    "latency":         latency,
                }
            }

            pt, err := client.NewPoint(
                "api_usage",
                tags,
                fields,
                start,
            )
            if err != nil {
                log.Println("Error: NewPoint failed, err=", err)
                return
            }
            msgQ <- pt
            log.Println("Add point.. ", pt)
            return
        }()
    }
}
