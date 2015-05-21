package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

const ACK = 0x6

type ConnState int

const (
	unconnected ConnState = iota
	connected
	looping
)

type VantageConn struct {
	address string
	conn    net.Conn
	buf     *bufio.Reader
	state   ConnState
}

func Dial(address string) (*VantageConn, error) {
	vc := &VantageConn{address: address}
	err := vc.connect()
	return vc, err
}

func (vc *VantageConn) connect() error {
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

func (vc *VantageConn) wakeup() error {
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

type CancelLoop struct{}

func (vc *VantageConn) Loop(times int, loopChan chan *LoopRecord) (chan CancelLoop, error) {
	err := vc.sendAckCommand(fmt.Sprintf("LOOP %v\n", times))
	if err != nil {
		return nil, fmt.Errorf("Error sending loop command: %v", err)
	}
	vc.state = looping
	cancel := make(chan CancelLoop, 1)
	go vc.loopRoutine(times, loopChan, cancel)
	// TODO cancel goroutine
	return cancel, nil
}

func (vc *VantageConn) loopRoutine(times int, loopChan chan *LoopRecord, cancelChan chan CancelLoop) {
	defer close(loopChan)
	pkt := make([]byte, 99)
	for i := 0; i < times; i++ {
		vc.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		c, err := io.ReadFull(vc.buf, pkt)
		if err != nil {
			log.Printf("Error during loop read: %v\n")
			if c > 0 {
				log.Printf("Got bytes: %v", pkt[:c])
			}
			return
		}
		loopChan <- parseLoop(pkt)
	}
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

func (vc *VantageConn) sendAckCommand(cmd string) error {
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

func main() {
	var host string
	flag.StringVar(&host, "h", "", "host:port of the Vantage device")
	flag.Parse()

	log.Printf("Connecting to %v...", host)
	vc, err := Dial(host)
	if err != nil {
		log.Fatalf("Error connecting: %v\n", err)
	}

	log.Printf("Looping...")
	loopChan := make(chan *LoopRecord, 100)
	_, err = vc.Loop(10, loopChan)
	if err != nil {
		log.Fatalf("Error from loop: %v\n", err)
	}
	for lr := range loopChan {
		log.Printf("Got Wind: %#v\n", lr)
	}
}
