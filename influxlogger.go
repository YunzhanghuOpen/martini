package martini

import (
    "fmt"
    "log"
    "net/http"
    "reflect"
    "strconv"
    "time"

    "github.com/influxdata/influxdb/client/v2"
    "redpacket"
)

// Logger returns a middleware handler that logs the request as it goes in and the response as it goes out.
func InfluxLogger() Handler {
    return func(res http.ResponseWriter, req *http.Request, c Context, log *log.Logger, msgQ chan *client.Point) {
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
        var token *redpacket.Token
        val := c.Get(reflect.TypeOf(token))
        if val.IsValid() {
            token = val.Interface().(*redpacket.Token)
        } else {
            token = nil
        }

        var errCode int32
        val = c.Get(reflect.TypeOf(errCode))
        if val.IsValid() {
            errCode = val.Interface().(int32)
        } else {
            errCode = -1111 // magic number
        }

        var receipt *redpacket.Receipt
        if req.URL.Path == `/api/hongbao/receive` {
            val = c.Get(reflect.TypeOf(receipt))
            if val.IsValid() {
                receipt = val.Interface().(*redpacket.Receipt)
            } else {
                receipt = nil
            }
        }

        go func() {
            var tags map[string]string
            var fields map[string]interface{}

            tags = map[string]string{
                "Req":       req.URL.Path,
                "IP":        addr,
                "RequestID": req.Header.Get("request-id"),
                "ErrCode":   fmt.Sprintf("%d", errCode),
            }

            fields = map[string]interface{}{
                "Status":  rw.Status(),
                "Latency": latency.Nanoseconds() / 1e3,
                "UA":      req.Header.Get("User-Agent"),
                "Version": req.Header.Get("version"),
            }

            if token != nil {
                tags["DealerID"] = fmt.Sprintf("%d", token.Base.DealerID)
                tags["BindUID"] = fmt.Sprintf("%d", token.Base.BindUID)
                tags["RealUID"] = fmt.Sprintf("%d", token.Ext.RealUID)
                tags["RedpacketUID"] = fmt.Sprintf("%d", token.Ext.RedpacketUID)

                fields["DealerUsername"] = token.Ext.DealerUsername
                // fields["dealer_code"] = fmt.Sprintf("%d", token.Ext.DealerCode)
            }

            // 默认无多个参数
            for k, v := range req.Form {
                fields[k] = v[0]
            }

            for k, v := range req.PostForm {
                fields[k] = v[0]
            }

            // TODO(yiyang): consider efficiency
            if req.URL.Path == `/api/hongbao/send` {
                if len(req.PostForm["Amount"]) > 0 {
                    fields["I_AMOUNT"], _ = strconv.ParseFloat(req.PostForm["Amount"][0], 64)
                }
                if len(req.PostForm["Count"]) > 0 {
                    fields["I_COUNT"], _ = strconv.ParseInt(req.PostForm["Count"][0], 10, 32)
                } else {
                    fields["I_COUNT"] = 1
                }
            }

            if receipt != nil { // `/api/hongbao/receive`
                fields["MyAmount"] = receipt.MyAmount
                fields["MyType"] = receipt.Type
                fields["MyRedpacketID"] = receipt.RedpacketID
                fields["SenderDealerUsername"] = receipt.ReceiveDetail.DealerUsername
            }
            delete(fields, "Avatar")
            delete(fields, "Message")

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
            return
        }()
    }
}
