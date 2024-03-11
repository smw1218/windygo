package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/smw1218/windygo/db"
	"github.com/smw1218/windygo/plot"
	"github.com/smw1218/windygo/raw"
	"github.com/smw1218/windygo/vantage"
)

type DataRecorder struct{}

func (dr *DataRecorder) record(loopRecord *vantage.LoopRecord) {
	log.Printf("R: %#v\n", loopRecord)
}

func main() {
	var host string
	var rawDir string
	var doDmp bool
	flag.StringVar(&host, "h", "", "host:port of the Vantage device")
	flag.StringVar(&rawDir, "raw", "", "directory to store raw data")
	flag.BoolVar(&doDmp, "dmp", false, "run archive dump and exit")
	flag.Parse()

	// On my unit, dmp didn't work (it was missing random bytes)
	// The DMPAFT worked but the data was all screwed up with dates jumping around
	// also some of the dates are in the future (multiple days)
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

	notifyChan := make(chan os.Signal, 1)
	signal.Notify(notifyChan, os.Interrupt, syscall.SIGTERM)

	db, err := db.NewMysql("windygo", "")
	if err != nil {
		log.Fatalln(err)
	}

	gp, err := plot.NewGnuPlot(db)
	if err != nil {
		log.Fatalln(err)
	}

	rawRecorder := raw.NewRecorder(rawDir)

	handler := func(loopPkt []byte) {
		db.Record(loopPkt)
		if rawDir != "" {
			rawRecorder.Record(loopPkt)
		}
	}

	go vantage.CollectDataForever(host, handler)
	for {
		select {
		case err1 := <-gp.ErrChan:
			log.Printf("GP error: %v\n", err1)
		case err2 := <-db.ErrChan:
			log.Printf("DB error: %v\n", err2)
		case <-notifyChan:
			log.Println("Shutting down")
			signal.Reset()
			rawRecorder.Shutdown()
			os.Exit(0)
		}
	}
}
