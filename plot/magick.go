package plot

import (
	"fmt"
	"github.com/smw1218/windygo/db"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// composite -dissolve 20 -geometry +150+150  Boardsports_on-top-01-300x93.png windreport.png watermarked.png

func addWatermark() error {
	cmd := exec.Command("composite", "-dissolve", "20", "-geometry", "+150+150", "Boardsports_on-top-01-300x93.png", "windreport.png", "watermarked.png")
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("Error running composite watermark: %v", err)
	}
	return nil
}

const currentCommand = `convert -size 100x400 canvas:white \
-font Roboto -pointsize 24 -fill 'rgb(30,115,190)' -draw 'text 5,25 "Wind"' \
-fill 'graya(50%, 0.5)' -draw 'line 0,30 100,30' \
-pointsize 16 -fill 'rgb(30,115,190)' -draw 'text 10,60 "Avg"' \
-pointsize 20 -fill black -draw 'text 10,90 "%0.1f"' \
-pointsize 16 -fill black -draw 'text 50,90 "%v"' \
-pointsize 16 -fill 'rgb(30,115,190)' -draw 'text 10,120 "Lull/Gust"' \
-pointsize 20 -fill black -draw 'text 10,150 "%0.0f/%0.0f"' \
-pointsize 24 -fill 'rgb(30,115,190)' -draw 'text 5,200 "Weather"' \
-fill 'graya(50%, 0.5)' -draw 'line 0,205 100,205' \
-pointsize 16 -fill 'rgb(30,115,190)' -draw 'text 10,235 "Temp"' \
-pointsize 20 -fill black -draw 'text 10,265 "%0.1f°"' \
-pointsize 16 -fill 'rgb(30,115,190)' -draw 'text 10,295 "Barometer"' \
-pointsize 20 -fill black -draw 'text 10,325 "%3f"' \
-pointsize 16 -fill 'rgb(30,115,190)' -draw 'text 10,355 "Humidity"' \
-pointsize 20 -fill black -draw 'text 10,385 "%v%"' \
-font CompassArrows -pointsize 20 -fill black -draw 'text 60,60 "%v"' \
current.png
`

var splitter = regexp.MustCompile(`[^\s']+|'[^']*'`)
var oneLineCmd = strings.Replace(currentCommand, "\\\n", "", -1)

func currentData(c *db.Summary) error {
	formatted := fmt.Sprintf(oneLineCmd,
		c.WindAvg,
		cardinalsText[c.WindDirAvgCardinal()],
		c.WindLull,
		c.WindGust,
		c.OutsideTempAvg,
		c.BarometerAvg,
		c.OutsideHumidityAvg)
	matches := splitter.FindAllStringSubmatch(formatted, -1)
	args := make([]string, len(matches))
	for i, match := range matches {
		args[i] = match[0]
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("Error running current: %v", err)
	}
	return nil
}
