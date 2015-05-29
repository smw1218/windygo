package vantage

import (
	"fmt"
	"io"
	"log"
	"sort"
	"time"
)

const PAGE_COUNT = 513
const PAGE_SIZE = 264
const RECORDS_PER_PAGE = 5
const DATA_RECORD_LENGTH = 52

// Rev B
type ArchiveRecord struct {
	ArchiveTime     time.Time
	OutsideTemp     float32
	HighOutsideTemp float32
	LowOutsideTemp  float32
	Rainfall        int
	HighRainRate    int
	Barometer       float32
	SolarRad        int
	WindSamples     int
	InsideTemp      float32
	InsideHumidity  int
	OutsideHumidity int
	WindAvg         int
	WindMax         int
	WindMaxDir      int
	WindDir         int
	UVIndexAvg      float32
	ET              float32
	HighSolarRad    int
	UVIndexMax      int
	ForecastRule    int
	LeafTemp        []int //2
	LeafWetness     []int //2
	SoilTemp        []int //4
	RecordType      int
	ExtraHumidities []int //2
	ExtraTemps      []int //3
	SoilMoistures   []int //4
}

type sortedArchive []*ArchiveRecord

func (sa sortedArchive) Len() int           { return len(sa) }
func (sa sortedArchive) Swap(i, j int)      { sa[i], sa[j] = sa[j], sa[i] }
func (sa sortedArchive) Less(i, j int) bool { return sa[i].ArchiveTime.Before(sa[j].ArchiveTime) }

func (vc *Conn) GetArchiveRecords() ([]*ArchiveRecord, error) {
	ars := make(sortedArchive, 0, PAGE_COUNT*RECORDS_PER_PAGE)
	archiveChan := make(chan *ArchiveRecord, 10)
	errChan := make(chan error, 1)

	err := vc.GetArchiveStream(archiveChan, errChan)
	if err != nil {
		return nil, err
	}
	for {
		select {
		case ar := <-archiveChan:
			if ar == nil {
				// Channel closed
				sort.Sort(ars)
				return ars, nil
			}
			ars = append(ars, ar)
		case err = <-errChan:
			return nil, err
		}
	}
}

func (vc *Conn) GetArchiveStream(archiveChan chan *ArchiveRecord, errChan chan error) error {
	err := vc.sendAckCommand("DMP\n")
	if err != nil {
		return fmt.Errorf("DMP command failed: %v", err)
	}
	go vc.dmpArchive(archiveChan, errChan)
	return nil
}

func (vc *Conn) dmpArchive(archiveChan chan *ArchiveRecord, errChan chan error) {
	pkt := make([]byte, PAGE_SIZE)
	_, err := vc.conn.Write([]byte{ACK})
	if err != nil {
		errChan <- fmt.Errorf("Error first DMP ACK: %v\n", err)
		return
	}
	j := 0
	for {
		vc.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := vc.conn.Read(pkt[j:])
		if err != nil {
			errChan <- fmt.Errorf("Err: %v\nDMP data: %v\n", err, pkt[:j+n])
			return
		}
		j += n
	}
	for i := 0; i < PAGE_COUNT; i++ {
		vc.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		c, err := io.ReadFull(vc.buf, pkt)
		if err != nil {
			if c > 0 {
				log.Printf("Got bytes: %v", pkt[:c])
			}
			errChan <- fmt.Errorf("Error during DMP read: %v\n", err)
			return
		}
		ars, err := parseArchive(pkt)
		if err != nil {
			//TODO
		}
		for _, ar := range ars {
			archiveChan <- ar
		}
		_, err = vc.conn.Write([]byte{ACK})
		if err != nil {
			errChan <- fmt.Errorf("Error during DMP ACK: %v\n", err)
			return
		}
	}
	close(archiveChan)
}

func parseArchive(pkt []byte) ([]*ArchiveRecord, error) {
	ret := make([]*ArchiveRecord, 0, 5)
	for i := 0; i < 5; i++ {
		dr := pkt[i*DATA_RECORD_LENGTH+1 : (i+1)*DATA_RECORD_LENGTH]
		tm := parseArchiveTime(toInt(dr[0], dr[1]), toInt(dr[2], dr[3]))
		if tm == (time.Time{}) {
			continue
		}
		// TODO CRC
		ar := &ArchiveRecord{
			ArchiveTime:     tm,
			OutsideTemp:     float32(toInt(dr[4], dr[5])) / 10,
			HighOutsideTemp: float32(toInt(dr[6], dr[7])) / 10,
			LowOutsideTemp:  float32(toInt(dr[8], dr[9])) / 10,
			Rainfall:        toInt(dr[10], dr[11]),
			HighRainRate:    toInt(dr[12], dr[13]),
			Barometer:       float32(toInt(dr[14], dr[15])) / 1000,
			SolarRad:        toInt(dr[16], dr[17]),
			WindSamples:     toInt(dr[18], dr[19]),
			InsideTemp:      float32(toInt(dr[20], dr[21])) / 10,
			InsideHumidity:  int(dr[22]),
			OutsideHumidity: int(dr[23]),
			WindAvg:         int(dr[24]),
			WindMax:         int(dr[25]),
			WindMaxDir:      archiveDirectionLookup[int(26)],
			WindDir:         archiveDirectionLookup[int(27)],
			UVIndexAvg:      float32(int(dr[28])) / 10,
			ET:              float32(int(dr[29])) / 1000,
			HighSolarRad:    toInt(dr[30], dr[31]),
			UVIndexMax:      int(dr[32]),
			ForecastRule:    int(dr[33]),
			LeafTemp:        nil,
			LeafWetness:     nil,
			SoilTemp:        nil,
			RecordType:      int(dr[42]),
			ExtraHumidities: nil,
			ExtraTemps:      nil,
			SoilMoistures:   nil,
		}
		ret = append(ret, ar)
	}
	return ret, nil
}

var archiveDirectionLookup map[int]int = map[int]int{
	0:   0,   // N
	1:   22,  // NNE
	2:   45,  // NE
	3:   67,  // ENE
	4:   90,  // E
	5:   112, // ESE
	6:   135, // SE
	7:   157, // SSE
	8:   180, // S
	9:   202, // SSW
	10:  225, // SW
	11:  247, // WSW
	12:  270, // W
	13:  292, // WNW
	14:  315, // NW
	15:  337, // NNW
	255: 0,
}

func parseArchiveTime(dt, tm int) time.Time {
	if dt == 0 {
		return time.Time{}
	}
	day := dt & 0x1f                     // lower 5 bits
	month := time.Month((dt >> 5) & 0xF) // 4 bits
	year := (dt >> 9) + 2000             // 7 bits
	hour := tm / 100
	min := tm - hour

	return time.Date(year, month, day, hour, min, 0, 0, time.Local)
}
