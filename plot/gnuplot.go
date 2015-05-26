package plot

import (
	"fmt"
	"github.com/smw1218/windygo/db"
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

type GnuPlot struct {
	saved []*db.Rollup
	last  *db.Rollup
}
