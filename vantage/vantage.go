package vantage

import (
	"bufio"
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

func (vc *Conn) Loop(times int, loopChan chan *LoopRecord, errChan chan error) error {
	err := vc.sendAckCommand(fmt.Sprintf("LOOP %v\n", times))
	if err != nil {
		return fmt.Errorf("Error sending loop command: %v", err)
	}
	vc.state = looping
	go vc.loopRoutine(times, loopChan, errChan)
	return nil
}

func (vc *Conn) loopRoutine(times int, loopChan chan *LoopRecord, errChan chan error) {
	pkt := make([]byte, 99)
	for i := 0; i < times; i++ {
		vc.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		c, err := io.ReadFull(vc.buf, pkt)
		if err != nil || (pkt[0] != byte('L') && pkt[1] != byte('O') && pkt[2] != byte('O')) {
			if c > 0 {
				log.Printf("Got bytes: %v", pkt[:c])
			}
			errChan <- fmt.Errorf("Error during loop read: %v\n", err)
			return
		}
		loopChan <- parseLoop(pkt)
	}
	errChan <- nil
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
	Recorded        time.Time
	Wind            int
	WindDirection   int
	WindAvg         int
	Barometer       float32
	BarTrend        string
	InsideTemp      float32
	OutsideTemp     float32
	InsideHumidity  int
	OutsideHumidity int
}

func parseLoop(pkt []byte) *LoopRecord {
	lr := &LoopRecord{
		Recorded:        time.Now(),
		Wind:            int(pkt[14]),
		WindDirection:   toInt(pkt[16], pkt[17]),
		WindAvg:         int(pkt[15]),
		Barometer:       float32(toInt(pkt[7], pkt[8])) / 1000,
		BarTrend:        BarTrendMap[pkt[3]],
		InsideTemp:      float32(toInt(pkt[9], pkt[10])) / 10,
		OutsideTemp:     float32(toInt(pkt[12], pkt[13])) / 10,
		InsideHumidity:  int(pkt[11]),
		OutsideHumidity: int(pkt[33]),
	}
	return lr
}
func toInt(lsb, msb byte) int {
	return int(lsb) | int(msb)<<8
}

func (vc *Conn) sendAckCommand(cmd string) error {
	_, err := vc.conn.Write([]byte(cmd))
	if err != nil {
		return fmt.Errorf("Error writing %v: %v", cmd, err)
	}
	buf := make([]byte, 1)
	vc.conn.SetReadDeadline(time.Now().Add(time.Second))
	n, err := vc.buf.Read(buf)
	if err != nil || n == 0 {
		return fmt.Errorf("Failed reading response to %v: %v", cmd, err)
	}
	if buf[0] != ACK {
		return fmt.Errorf("Unknown response from %v: %v", cmd, buf)
	}
	return nil
}

func (v *Conn) Close() error {
	return v.conn.Close()
}

type LoopHandler func(rec *LoopRecord)

func CollectDataForever(host string, handler LoopHandler) {
	var vc *Conn
	var err error
	loopChan := make(chan *LoopRecord, 100)
	go loopRoutine(loopChan, handler)
	errChan := make(chan error, 1)
	for {
		log.Printf("Connecting to %v...", host)
		vc, err = Dial(host)
		for err != nil {
			log.Printf("Error connecting: %v\n", err)
			log.Printf("Connect Retry in 60 seconds")
			time.Sleep(time.Minute)
			log.Printf("Connecting to %v...", host)
			vc, err = Dial(host)
		}

		for err == nil {
			log.Printf("Looping 60 times")
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

func loopRoutine(loopChan chan *LoopRecord, handler LoopHandler) {
	for lr := range loopChan {
		handler(lr)
	}
}
