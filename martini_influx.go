// Package martini is a powerful package for quickly writing modular web applications/services in Golang.
//
// For a full guide visit http://github.com/go-martini/martini
//
//  package main
//
//  import "github.com/go-martini/martini"
//
//  func main() {
//    m := martini.Classic()
//
//    m.Get("/", func() string {
//      return "Hello world!"
//    })
//
//    m.Run()
//  }
package martini

import (
    "github.com/YunzhanghuOpen/glog"
    "github.com/influxdata/influxdb/client/v2"
)

const (
    DB_NAME string = "ApiReport"
    BATCH_SIZE int = 15
    CHAN_CAP int = 1000

)
// needs to create database systemstats
func ReportPoint(c client.Client, msgsQ chan *client.Point) {
    for {
        bp, err := client.NewBatchPoints(client.BatchPointsConfig{
            Database:  DB_NAME,
            Precision: "s",
        })
        if err != nil {
            glog.Warning("Error: NewBatchPoints err=", err.Error())
        }

        for i := 0; i < BATCH_SIZE; i++ {
            pt := <-msgsQ
            glog.V(30).Info("Add Point...", pt)
            bp.AddPoint(pt)
        }

        glog.V(30).Infof("Sending %d Point to influxdb...\n", BATCH_SIZE)
        if err = c.Write(bp); err != nil {
            glog.Warning("Error Write influxdb err=", err.Error())
        }
    }
}

// Add msgQ and for msg report
func InfluxM(c client.Client) *ClassicMartini {
    r := NewRouter()
    m := New()
    var msgQ = make(chan *client.Point, CHAN_CAP)
    m.Map(msgQ)
    m.Use(InfluxLogger())
    m.Use(Recovery())
    m.Use(Static("public"))
    m.MapTo(r, (*Routes)(nil))
    m.Action(r.Handle)
    go ReportPoint(c, msgQ)
    return &ClassicMartini{m, r}
}
