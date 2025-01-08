package plot

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/smw1218/windygo/db"
)

/*
const currentCommand = `convert -size 100x400 canvas:white \
-font Roboto -pointsize 24 -fill 'rgb(30,115,190)' -draw 'text 5,25 "Wind"' \
-fill 'graya(50%%, 0.5)' -draw 'line 0,30 100,30' \
-pointsize 16 -fill 'rgb(30,115,190)' -draw 'text 10,60 "Avg"' \
-pointsize 20 -fill black -draw 'text 10,90 "%0.1f"' \
-pointsize 16 -fill black -draw 'text 50,90 "%v"' \
-pointsize 16 -fill 'rgb(30,115,190)' -draw 'text 10,120 "Lull/Gust"' \
-pointsize 20 -fill black -draw 'text 10,150 "%0.0f/%0.0f"' \
-pointsize 24 -fill 'rgb(30,115,190)' -draw 'text 5,200 "Weather"' \
-fill 'graya(50%%, 0.5)' -draw 'line 0,205 100,205' \
-pointsize 16 -fill 'rgb(30,115,190)' -draw 'text 10,235 "Temp"' \
-pointsize 20 -fill black -draw 'text 10,265 "%0.1f°"' \
-pointsize 16 -fill 'rgb(30,115,190)' -draw 'text 10,295 "Barometer"' \
-pointsize 20 -fill black -draw 'text 10,325 "%0.3f"' \
-pointsize 16 -fill 'rgb(30,115,190)' -draw 'text 10,355 "Humidity"' \
-pointsize 20 -fill black -draw 'text 10,385 "%v%%"' \
-font CompassArrows -pointsize 20 -fill black -draw 'text 60,60 "%v"' \
current.png
`
*/

const currentCommand = `convert -size 100x400 canvas:white \
-font Roboto -pointsize 24 -fill 'rgb(30,115,190)' -draw 'text 5,25 "Wind"' \
-fill 'graya(50%%, 0.5)' -draw 'line 0,30 100,30' \
-pointsize 16 -fill 'rgb(30,115,190)' -draw 'text 10,60 "Avg"' \
-pointsize 20 -fill black -draw 'text 10,85 "%0.1f"' \
-pointsize 14 -fill black -draw 'text 10,105 "%v %v°"' \
-pointsize 16 -fill 'rgb(30,115,190)' -draw 'text 10,130 "Lull/Gust"' \
-pointsize 20 -fill black -draw 'text 10,155 "%0.1f/%0.1f"' \
-pointsize 24 -fill 'rgb(30,115,190)' -draw 'text 5,200 "Weather"' \
-fill 'graya(50%%, 0.5)' -draw 'line 0,205 100,205' \
-pointsize 16 -fill 'rgb(30,115,190)' -draw 'text 10,230 "Temp"' \
-pointsize 20 -fill black -draw 'text 10,255 "%0.1f°"' \
-pointsize 14 -fill black -draw 'text 10,275 "%0.1fC"' \
-pointsize 16 -fill 'rgb(30,115,190)' -draw 'text 10,295 "Barometer"' \
-pointsize 20 -fill black -draw 'text 10,320 "%0.3f"' \
-pointsize 16 -fill 'rgb(30,115,190)' -draw 'text 10,355 "Humidity"' \
-pointsize 20 -fill black -draw 'text 10,380 "%v%%"' \
-font CompassArrows -pointsize 20 -fill black -draw 'text 50,85 "%v"' \
-font %v -pointsize 20 -fill black -draw 'text 75,320 "%v"' \
current.png
`

var splitter = regexp.MustCompile(`[^\s']+|'[^']*'`)
var oneLineCmd = strings.Replace(currentCommand, "\\\n", "", -1)

// currentData creates an image that shows the summary of the current
// minute. It uses ImageMagick to create a png file that is then
// composited with the graph in the finish script.
func currentData(c *db.Summary) error {
	formatted := fmt.Sprintf(oneLineCmd,
		c.WindAvg,
		cardinalsText[c.WindDirAvgCardinal()],
		c.WindDirectionAvg,
		c.WindLull,
		c.WindGust,
		c.OutsideTempAvg,
		c.OutsideTempAvgCelsius(),
		c.BarometerAvg,
		c.OutsideHumidityAvg,
		cardinals[c.WindDirAvgCardinal()],
		barTrendFont[c.BarTrendByte],
		barTrendMap[c.BarTrendByte])

	//log.Printf("command: %v", formatted)
	matches := splitter.FindAllStringSubmatch(formatted, -1)
	args := make([]string, len(matches))
	for i, match := range matches {
		args[i] = strings.Replace(match[0], "'", "", -1)
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error running current %v: %w", formatted, err)
	}
	return nil
}

func finishReport() error {
	cmd := exec.Command("./finish.sh")
	cmd.Stderr = os.Stderr
	doneChan := make(chan struct{}, 1)
	go finishKiller(cmd, doneChan)
	err := cmd.Run()
	doneChan <- struct{}{}
	if err != nil {
		return fmt.Errorf("error running finish: %w", err)
	}
	return nil
}

// Kills the finish script after 60 seconds
func finishKiller(cmd *exec.Cmd, doneChan chan struct{}) {
	waitChan := time.After(60 * time.Second)
	select {
	case <-waitChan:
		cmd.Process.Kill()
	case <-doneChan:
	}
}
