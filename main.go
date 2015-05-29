package main

import (
	"flag"
	"fmt"
	"github.com/smw1218/windygo/db"
	"github.com/smw1218/windygo/plot"
	"github.com/smw1218/windygo/vantage"
	"log"
)

type DataRecorder struct{}

func (dr *DataRecorder) record(loopRecord *vantage.LoopRecord) {
	log.Printf("R: %#v\n", loopRecord)
}

func main() {
	var host string
	var doDmp bool
	flag.StringVar(&host, "h", "", "host:port of the Vantage device")
	flag.BoolVar(&doDmp, "dmp", false, "run archive dump and exit")
	flag.Parse()

	if doDmp {
		vc, err := vantage.Dial(host)
		if err != nil {
			log.Fatalf("Error connecting to vantage: %v", err)
		}

		ars, err := vc.GetArchiveRecords()
		if ars != nil {
			for _, ar := range ars {
				fmt.Printf("I:%v\tJ:%v\t%v\t%v\t%v\n", ar.ArchivePage, ar.ArchivePageRecord, ar.ArchiveTime, ar.WindAvg, ar.OutsideTemp)
			}
		}
		if err != nil {
			log.Fatalf("Error getting archive: %v\n", err)
		}

		return
	}
	db, err := db.NewMysql("windygo", "")
	if err != nil {
		log.Fatal(err)
	}

	gp, err := plot.NewGnuPlot(db)
	if err != nil {
		log.Fatal(err)
	}

	go vantage.CollectDataForever(host, db.Record)
	for {
		select {
		case err := <-gp.ErrChan:
			log.Printf("GP error: %v\n", err)
		case err := <-db.ErrChan:
			log.Printf("DB error: %v\n", err)
		}
	}
}
