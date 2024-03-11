package raw

import (
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/smw1218/windygo/vantage"
)

type Recorder struct {
	baseDir     string
	writeMutex  sync.Mutex
	currentFile *os.File
}

func NewRecorder(baseDir string) *Recorder {
	return &Recorder{
		baseDir: baseDir,
	}
}

func (r *Recorder) Shutdown() {
	// close file
	// don't release the lock here, we don't want to
	// try writing after close
	r.writeMutex.Lock()
	if r.currentFile == nil {
		return
	}
	err := r.currentFile.Close()
	if err != nil {
		log.Printf("Error closing file %v: %v", r.currentFile.Name(), err)
	}
}

func (r *Recorder) Record(loopPkt []byte) {
	loopRecord := vantage.ParseLoop(loopPkt)
	r.writeMutex.Lock()
	defer r.writeMutex.Unlock()
	fn := r.fileName(loopRecord.Recorded)
	err := r.ensureCurrentFile(fn)
	if err != nil {
		log.Println(err)
		return
	}
	_, err = r.currentFile.Write(loopPkt)
	if err != nil {
		log.Printf("Error writing to file %v: %v", fn, err)
		return
	}
	err = r.currentFile.Sync()
	if err != nil {
		log.Printf("Error syncing file %v: %v", fn, err)
		return
	}
}

func (r *Recorder) ensureCurrentFile(fileName string) error {
	if r.currentFile == nil || fileName != r.currentFile.Name() {
		if r.currentFile != nil {
			// close out current file
			err := r.currentFile.Close()
			if err != nil {
				log.Printf("Error closing file %v: %v", r.currentFile.Name(), err)
			}
		}

		newFile, err := r.open(fileName)
		if err != nil {
			return fmt.Errorf("error opening file %v: %v", fileName, err)
		}
		r.currentFile = newFile
	}
	return nil
}

func (r *Recorder) open(fileName string) (*os.File, error) {
	dir := path.Dir(fileName)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, fmt.Errorf("error creating directory: %w", err)
	}
	return os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
}

// baseDir/<year>/<month>/<day>/<hour>.rec
func (r *Recorder) fileName(now time.Time) string {
	return path.Join(r.baseDir,
		strconv.Itoa(now.Year()),
		strconv.Itoa(int(now.Month())),
		strconv.Itoa(now.Day()),
		fmt.Sprintf("%d.rec", now.Hour()),
	)
}
