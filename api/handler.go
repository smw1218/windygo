package api

import (
	"net/http"
	"os"
	"time"

	"github.com/smw1218/windygo/db"
	"github.com/smw1218/windygo/plot"
)

func CreateRoutes(mysql *db.Mysql) http.Handler {
	muxer := http.NewServeMux()
	plotter := NewPlotter(mysql)
	muxer.HandleFunc("/plot", plotter.FullPlot)
	return muxer
}

type Plotter struct {
	mysql *db.Mysql
}

func NewPlotter(mysql *db.Mysql) *Plotter {
	return &Plotter{mysql: mysql}
}

func (p *Plotter) FullPlot(w http.ResponseWriter, r *http.Request) {
	reportSize := 12 * time.Hour
	var startTime time.Time
	var err error

	startQp := r.URL.Query().Get("start")
	if startQp == "" {
		startTime = time.Now().Add(-reportSize)
	} else {
		startTime, err = time.Parse("2006-01-02T15:04:05", startQp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	summaries, err := p.mysql.GetSummaries(startTime, reportSize, 300)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = plot.CreateFullReport(summaries, summaries[len(summaries)-1])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	f, err := os.Open("windreport.png")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()
	http.ServeContent(w, r, "windreport", time.Now(), f)
}
