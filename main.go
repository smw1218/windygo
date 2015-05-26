package main

import (
	"flag"
	"github.com/smw1218/windygo/db"
	"github.com/smw1218/windygo/vantage"
	"log"
)

type DataRecorder struct{}

func (dr *DataRecorder) record(loopRecord *vantage.LoopRecord) {
	log.Printf("R: %#v\n", loopRecord)
}

func main() {
	var host string
	flag.StringVar(&host, "h", "", "host:port of the Vantage device")
	flag.Parse()

	db, err := db.NewMysql("windygo", "")
	if err != nil {
		log.Fatal(err)
	}
	go vantage.CollectDataForever(host, db.Record)
	for {
		select {
		case lastSaved := <-db.SavedChan:
			if lastSaved.SummarySeconds == 60 {
				//TODO make a pretty graph
			}
		case err := <-db.ErrChan:
			log.Printf("DB error: %v\n", err)
		}
	}
}
