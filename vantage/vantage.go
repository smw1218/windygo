package vantage

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

const ACK = 0x6
const NACK = 0x21
const CANCEL = 0x18
const DMPNACK = 0x15
const ESC = 0x1b

const (
	LOOP_PACKET_SIZE = 99
	LOOP_RECORD_SIZE = LOOP_PACKET_SIZE + 8
)

type ConnState int

const (
	unconnected ConnState = iota
	connected
	looping
)

type Conn struct {
	address string
	conn    net.Conn
	buf     *bufio.Reader
	state   ConnState
}

func Dial(address string) (*Conn, error) {
	vc := &Conn{address: address}
	err := vc.connect()
	return vc, err
}

func (vc *Conn) connect() error {
	var err error
	vc.conn, err = net.Dial("tcp", vc.address)
	if err != nil {
		return fmt.Errorf("Error dialing: %v\n", err)
	}
	vc.state = unconnected
	// flush any data
	// needed?
	/*
		dump := make([]byte, 100)
		for err == nil {
			vc.conn.SetReadDeadline(time.Now().Add(time.Second))
			var n int
			n, err = vc.conn.Read(dump)
			if err == nil {
				log.Printf("Dumping %v bytes", n)
			}
		}
	*/
	return vc.wakeup()
}

func (vc *Conn) wakeup() error {
	for i := 0; i < 3; i++ {
		fmt.Fprintf(vc.conn, "\n")
		// docs specifically reccomend 1.2s of wait for response
		vc.conn.SetReadDeadline(time.Now().Add(1200 * time.Millisecond))
		buf := make([]byte, 2)
		i, err := vc.conn.Read(buf)
		if err != nil {
			log.Printf("Error waking console: %v read %v bytes\n", err, i)
		} else {
			if string(buf) == "\n\r" {
				vc.state = connected
				break
			} else {
				log.Printf("Got invalid response: %v", buf)
			}
		}
	}
	if vc.state != connected {
		return fmt.Errorf("Failed to wake after 3 connection attempts")
	}
	vc.conn.SetReadDeadline(time.Time{})
	vc.buf = bufio.NewReader(vc.conn)
	return nil
}

func (vc *Conn) Loop(times int, loopChan chan []byte, errChan chan error) error {
	err := vc.sendAckCommand(fmt.Sprintf("LOOP %v\n", times))
	if err != nil {
		return fmt.Errorf("error sending loop command: %w", err)
	}
	vc.state = looping
	go vc.loopRoutine(times, loopChan, errChan)
	return nil
}

func (vc *Conn) loopRoutine(times int, loopChan chan []byte, errChan chan error) {
	pkt := make([]byte, LOOP_PACKET_SIZE+8)
	for i := 0; i < times; i++ {
		vc.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		c, err := io.ReadFull(vc.buf, pkt[8:])
		if err != nil {
			if c > 0 {
				log.Printf("Got bytes: %v", pkt[:c])
			}
			errChan <- fmt.Errorf("error during loop read: %w", err)
			return
		}
		now := time.Now().UnixNano()
		binary.LittleEndian.PutUint64(pkt, uint64(now))
		if err = validateLoop(pkt[8:]); err != nil {
			log.Printf("Validate error: %#v\n", pkt)
			errChan <- err
		} else {
			loopChan <- pkt
		}
	}
	errChan <- nil
}

func validateLoop(pkt []byte) error {
	if pkt[0] != byte('L') && pkt[1] != byte('O') && pkt[2] != byte('O') {
		return fmt.Errorf("Loop doesn't begin with 'LOO'")
	}
	crcCalc := int(crcData(pkt[0 : LOOP_PACKET_SIZE-2]))
	crcSent := toInt(pkt[LOOP_PACKET_SIZE-1], pkt[LOOP_PACKET_SIZE-2])
	if crcCalc != crcSent {
		return fmt.Errorf("LOOP CRC check failed")
	}
	return nil
}

var BarTrendMap = map[byte]string{
	196: "Falling Rapidly",
	236: "Falling Slowly",
	0:   "Steady",
	20:  "Rising Slowly",
	60:  "Rising Rapidly",
	80:  "Unknown",
}

type LoopRecord struct {
	// TODO everything else
	Recorded        time.Time `sql:"index"`
	Wind            int       // mph
	WindDirection   int       // degrees
	WindAvg         int       // mph
	BarometerRaw    int       // in Hg/1000
	BarTrend        string
	BarTrendByte    byte
	InsideTempRaw   int // 1/10 F
	OutsideTempRaw  int // 1/10 F
	InsideHumidity  int // %
	OutsideHumidity int // %
	RainRateRaw     int // clicks/hr click == 0.01in
	//UV              int
	//SolarRadiation  int // watt/m^2
	StormRainRaw int // in
	StartOfStorm time.Time
	DayRainRaw   int
	MonthRainRaw int
	YearRainRaw  int
}

func ParseLoop(pktFull []byte) *LoopRecord {
	recorded := time.Unix(0, int64(binary.LittleEndian.Uint64(pktFull)))
	pkt := pktFull[8:]
	lr := &LoopRecord{
		Recorded:        recorded,
		Wind:            int(pkt[14]),
		WindDirection:   toInt(pkt[16], pkt[17]),
		WindAvg:         int(pkt[15]),
		BarometerRaw:    toInt(pkt[7], pkt[8]),
		BarTrend:        BarTrendMap[pkt[3]],
		BarTrendByte:    pkt[3],
		InsideTempRaw:   toInt(pkt[9], pkt[10]),
		OutsideTempRaw:  toInt(pkt[12], pkt[13]),
		InsideHumidity:  int(pkt[11]),
		OutsideHumidity: int(pkt[33]),
		RainRateRaw:     toInt(pkt[41], pkt[42]),
		//UV:              int(pkt[43]),
		//SolarRadiation:  toInt(pkt[44], pkt[45]),
		StormRainRaw: toInt(pkt[46], pkt[47]),
		StartOfStorm: startOfStorm(pkt[48], pkt[49]),
		DayRainRaw:   toInt(pkt[50], pkt[51]),
		MonthRainRaw: toInt(pkt[52], pkt[53]),
		YearRainRaw:  toInt(pkt[54], pkt[55]),
	}
	return lr
}
func toInt(lsb, msb byte) int {
	return int(lsb) | int(msb)<<8
}

func startOfStorm(lsb, msb byte) time.Time {
	rawValue := uint(lsb) | uint(msb)<<8
	// Bit 15 to bit 12 is the month,
	month := time.Month(rawValue >> 12)
	// Bit 11 to bit 7 is the day
	day := int(rawValue >> 7 & 0x1F)
	// Bit 6 to bit 0 is the year offseted by 2000.
	year := int(rawValue&0x3F) + 2000

	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func (lr *LoopRecord) Barometer() float32 {
	return float32(lr.BarometerRaw) / 1000
}

func (lr *LoopRecord) TempConversion(raw int) float32 {
	return float32(raw) / 10
}

func (lr *LoopRecord) RainConversion(raw int) float32 {
	return float32(raw) / 100
}

func (lr *LoopRecord) InsideTemp() float32 {
	return lr.TempConversion(lr.InsideTempRaw)
}

func (lr *LoopRecord) OutsideTemp() float32 {
	return lr.TempConversion(lr.OutsideTempRaw)
}

func (lr *LoopRecord) RainRate() float32 {
	return lr.RainConversion(lr.RainRateRaw)
}

func (lr *LoopRecord) StormRain() float32 {
	return lr.RainConversion(lr.StormRainRaw)
}

func (lr *LoopRecord) DayRain() float32 {
	return lr.RainConversion(lr.DayRainRaw)
}

func (vc *Conn) sendAckCommand(cmd string) error {
	_, err := vc.conn.Write([]byte(cmd))
	if err != nil {
		return fmt.Errorf("error writing %v: %w", cmd, err)
	}
	buf := make([]byte, 1)
	vc.conn.SetReadDeadline(time.Now().Add(time.Second))
	n, err := vc.buf.Read(buf)
	if err != nil || n == 0 {
		return fmt.Errorf("failed reading response to %v: %w", cmd, err)
	}
	if buf[0] != ACK {
		return fmt.Errorf("unknown response from %v: %v", cmd, buf)
	}
	return nil
}

func (v *Conn) Close() error {
	return v.conn.Close()
}

type LoopHandler func(loopPkt []byte)

func CollectDataForever(host string, handler LoopHandler) {
	var vc *Conn
	var err error
	loopChan := make(chan []byte, 100)
	go loopRoutine(loopChan, handler)
	errChan := make(chan error, 1)
	for {
		log.Printf("Connecting to %v...", host)
		vc, err = Dial(host)
		for err != nil {
			log.Printf("Error connecting: %v", err)
			log.Printf("Connect Retry in 60 seconds")
			time.Sleep(time.Minute)
			log.Printf("Connecting to %v...", host)
			vc, err = Dial(host)
		}

		log.Printf("Connected to %v", host)
		for err == nil {
			//log.Printf("Looping 60 times")
			err = vc.Loop(60, loopChan, errChan)
			if err != nil {
				log.Printf("Error from loop: %v\n", err)
				vc.Close()
				break
			}
			err = <-errChan
			if err != nil {
				vc.Close()
			}
		}
	}
}

func loopRoutine(loopChan chan []byte, handler LoopHandler) {
	for lr := range loopChan {
		handler(lr)
	}
}
