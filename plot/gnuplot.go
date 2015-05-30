package plot

import (
	//"bufio"
	"fmt"
	"github.com/smw1218/windygo/db"
	"io"
	"log"
	"os"
	"os/exec"
	"time"
)

// mapping of direction to custom font
var cardinals map[int]string = map[int]string{
	0:   "", // N 		f100
	22:  "", // NNE	f101
	45:  "", // NE		f102
	67:  "", // ENE	f103
	90:  "", // E		f104
	112: "", // ESE	f105
	135: "", // SE		f106
	157: "", // SSE	f107
	180: "", // S		f108
	202: "", // SSW	f109
	225: "", // SW		f10a
	247: "", // WSW	f10b
	270: "", // W		f10c
	292: "", // WNW	f10d
	315: "", // NW		f10e
	337: "", // NNW	f10f
}

const summarySecondsForGraph = 300
const summarySecondsForGeneration = 60
const gpFormat = "2006-01-02_15:04:05"

// Mon Jan 2 15:04:05 -0700 MST 2006

type GnuPlot struct {
	// TODO mutex
	saved         []*db.Summary
	nextSave      int
	currentMinute *db.Summary
	summaryChan   chan *db.Summary
	ErrChan       chan error
}

func NewGnuPlot(mysql *db.Mysql) (*GnuPlot, error) {
	gp := &GnuPlot{}
	var err error
	gp.saved, err = mysql.GetSummaries(12*time.Hour, summarySecondsForGraph)
	if err != nil {
		return nil, err
	}
	gp.summaryChan = mysql.SavedChan
	gp.ErrChan = make(chan error, 5)
	go gp.generator()
	return gp, nil
}

func (gp *GnuPlot) generator() {
	for summary := range gp.summaryChan {
		if summary.SummarySeconds == summarySecondsForGraph {
			gp.saved[gp.nextSave%len(gp.saved)] = summary
			gp.nextSave++
		} else if summary.SummarySeconds == summarySecondsForGeneration {
			gp.currentMinute = summary
			gp.createPlot()
		}
	}
}

func (gp *GnuPlot) createPlot() {
	log.Printf("Creating plot")
	cmd := exec.Command("gnuplot")
	rd, wr := io.Pipe()
	cmd.Stdin = rd
	cmd.Stderr = os.Stderr
	var toWrite io.Writer = wr
	f, err := os.OpenFile("gpuplot_input.data", os.O_CREATE|os.O_WRONLY, 0)
	if err != nil {
		gp.sendError(fmt.Errorf("Error running gnuplot: %v", err))
	} else {
		toWrite = io.MultiWriter(wr, f)
	}
	go gp.writeData(toWrite, wr)
	err = cmd.Run()
	if err != nil {
		gp.sendError(fmt.Errorf("Error running gnuplot: %v", err))
	}
}

func (gp *GnuPlot) sendError(err error) {
	select {
	case gp.ErrChan <- err:
	default:
		log.Printf("Plot Error: %v\n", err)
	}
}

func (gp *GnuPlot) writeData(w io.Writer, closeme *io.PipeWriter) {
	defer closeme.Close()
	//write the script first
	_, err := io.WriteString(w, gnuPlotScript)
	if err != nil {
		gp.sendError(fmt.Errorf("Process write err: %v", err))
	}

	lensaved := len(gp.saved)
	// write the avg data
	for i := 0; i < lensaved; i++ {
		summary := gp.saved[(i+gp.nextSave)%lensaved]
		if summary != nil {
			_, err = io.WriteString(w, fmt.Sprintf("%s\t%v\n", summary.EndTime.Format(gpFormat), summary.WindAvg))
			if err != nil {
				gp.sendError(fmt.Errorf("Process write err: %v", err))
			}
		}
	}
	io.WriteString(w, "e\n")

	// write the lull data
	for i := 0; i < lensaved; i++ {
		summary := gp.saved[(i+gp.nextSave)%lensaved]
		if summary != nil {
			_, err = io.WriteString(w, fmt.Sprintf("%s\t%v\n", summary.EndTime.Format(gpFormat), summary.WindLull))
			if err != nil {
				gp.sendError(fmt.Errorf("Process write err: %v", err))
			}
		}
	}
	io.WriteString(w, "e\n")

	// write the gust data
	for i := 0; i < lensaved; i++ {
		summary := gp.saved[(i+gp.nextSave)%lensaved]
		if summary != nil {
			_, err = io.WriteString(w, fmt.Sprintf("%s\t%v\n", summary.EndTime.Format(gpFormat), summary.WindGust))
			if err != nil {
				gp.sendError(fmt.Errorf("Process write err: %v", err))
			}
		}
	}
	io.WriteString(w, "e\n")

	// write the direction data
	for i := 0; i < lensaved; i++ {
		summary := gp.saved[(i+gp.nextSave)%lensaved]
		if summary != nil && summary.EndTime.Equal(summary.EndTime.Truncate(15*time.Minute)) {
			_, err = io.WriteString(w, fmt.Sprintf("%s\t%v\n", summary.EndTime.Format(gpFormat), cardinals[summary.WindDirAvgCardinal()]))
			if err != nil {
				gp.sendError(fmt.Errorf("Process write err: %v", err))
			}
		}
	}
	io.WriteString(w, "e\n")
}

const gnuPlotScript = `
set encoding utf8
set term png size 600, 400 truecolor enhanced
set output "windreport.png"
set xdata time
set timefmt "%Y-%m-%d_%H:%M:%S"
set format x "%l%p\n%m/%d"
set autoscale xfixmin
set autoscale xfixmax
set autoscale x2fixmin
set autoscale x2fixmax
set xtics 3600 rangelimited
set mxtics 4
set grid xtics ytics mxtics
set style fill transparent solid 0.50 noborder
set arrow size 5, 45 front
plot [] [0:30<*] "-" using 1:2 title "Wind Avg (mph)" with filledcurves y1=0, \
 "-" using 1:2 title "Wind Lull" with lines \
 "-" using 1:2 title "Wind Gust" with lines \
 "-" using 1:(50):2 title "" with labels font "compass-arrows,24"
`
