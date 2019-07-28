package plot

import (
	"testing"

	"github.com/smw1218/windygo/db"
)

func TestMagick(t *testing.T) {
	summ := &db.Summary{
		WindAvg:            15.5,
		WindGust:           20.2,
		WindLull:           10.3,
		WindDirectionAvg:   250,
		BarometerAvg:       29.995,
		OutsideTempAvg:     75.1,
		OutsideHumidityAvg: 73,
		BarTrendByte:       20,
	}
	err := currentData(summ)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
}
