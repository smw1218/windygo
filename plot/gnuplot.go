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
/*
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
*/

//* for debugging font issues
// gnuplot or libgd on raspian doesn't seem to support
// the extended unicode range
var cardinals map[int]string = map[int]string{
	0:   "0", // N 		f100
	22:  "1", // NNE	f101
	45:  "2", // NE		f102
	67:  "3", // ENE	f103
	90:  "4", // E		f104
	112: "5", // ESE	f105
	135: "6", // SE		f106
	157: "7", // SSE	f107
	180: "8", // S		f108
	202: "9", // SSW	f109
	225: "a", // SW		f10a
	247: "b", // WSW	f10b
	270: "c", // W		f10c
	292: "d", // WNW	f10d
	315: "e", // NW		f10e
	337: "f", // NNW	f10f
}

var cardinalsText map[int]string = map[int]string{
	0:   "N",   // N 		f100
	22:  "NNE", // NNE	f101
	45:  "NE",  // NE		f102
	67:  "ENE", // ENE	f103
	90:  "E",   // E		f104
	112: "ESE", // ESE	f105
	135: "SE",  // SE		f106
	157: "SSE", // SSE	f107
	180: "S",   // S		f108
	202: "SSW", // SSW	f109
	225: "SW",  // SW		f10a
	247: "WSW", // WSW	f10b
	270: "W",   // W		f10c
	292: "WNW", // WNW	f10d
	315: "NW",  // NW		f10e
	337: "NNW", // NNW	f10f
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
	f, err := os.OpenFile("gnuplot_input.data", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0664)
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

// TODO make template for changing things
const gnuPlotScript = `
set encoding utf8
set term png size 600, 400 truecolor enhanced font "RobotoCondensed"
set output "windreport.png"
set tmargin 2
set label "Alameda" at graph 0,1.03 left font ",24"
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
set style fill transparent solid 0.30 
set style line 1 lt rgb "blue" lw 2 pt 0
set style line 2 lt rgb "sea-green" lw 2 pt 0
set style line 3 lt rgb "dark-red" lw 2 pt 0
plot [] [0:40<*] "-" using 1:2 title "Wind Avg (mph)" with filledcurves y1=0, \
 "-" using 1:2 title "Wind Lull" with lines lw 2, \
 "-" using 1:2 title "Wind Gust" with lines lw 2, \
 "-" using 1:(26):2 title "" with labels font "CompassArrows,24"
`
