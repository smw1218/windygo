package db

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/smw1218/windygo/vantage"
)

var Intervals = []time.Duration{time.Minute, 5 * time.Minute, 10 * time.Minute}

const DegreesPerRadian = 180 / math.Pi
const RadiansPerDegree = math.Pi / 180
const summariesTable string = `
CREATE TABLE IF NOT EXISTS summaries (
	id 						integer AUTO_INCREMENT PRIMARY KEY,
	start_time				timestamp,
	end_time				timestamp,
	measurments				integer,
	summary_seconds			integer,
	wind_avg				float,
	wind_gust				float,
	wind_lull				float,
	wind_stddev				float,
	wind_direction_avg		integer,
	wind_direction_min		integer,
	wind_direction_max		integer,
	barometer_avg			float,
	barometer_start			float,
	outside_temp_avg		float,
	outside_humidity_avg	float,
	INDEX end_time_idx (end_time),
	INDEX summary_minutes_idx (summary_seconds)
)
`

type LoopRecord struct {
	ID uint `gorm:"primary_key"`
	vantage.LoopRecord
}

type Summary struct {
	ID                 int64
	StartTime          time.Time
	EndTime            time.Time
	Measurements       int64
	SummarySeconds     int64
	WindAvg            float64
	WindGust           float64
	WindLull           float64
	WindStddev         float64
	WindDirectionAvg   int64
	WindDirectionMin   int64
	WindDirectionMax   int64
	BarometerAvg       float64
	BarometerStart     float64
	OutsideTempAvg     float64
	OutsideHumidityAvg float64
	BarTrendByte       byte
}

func (s *Summary) WindDirAvgCardinal() int {
	tmp := int(math.Floor(float64(s.WindDirectionAvg)/22.5+.5) * 22.5)
	if tmp == 360 {
		tmp = 0
	}
	return tmp
}

func (s *Summary) OutsideTempAvgCelsius() float64 {
	return (s.OutsideTempAvg - 32) * 5 / 9
}

func (s *Summary) Valid() bool {
	return s.WindAvg < 100 && s.WindGust < 100
}

const insertSql string = `insert into summaries (%v) VALUES (%v)`

var insertCols []string = []string{
	"start_time", "end_time", "measurments", "summary_seconds", "wind_avg",
	"wind_gust", "wind_lull", "wind_stddev", "wind_direction_avg",
	"wind_direction_min", "wind_direction_max", "barometer_avg",
	"barometer_start", "outside_temp_avg", "outside_humidity_avg",
}

type Mysql struct {
	DB         *sql.DB
	SavedChan  chan *Summary
	ErrChan    chan error
	rollups    []*Rollup
	insertStmt *sql.Stmt
	ORM        *gorm.DB
}

type Rollup struct {
	Period             time.Time
	Interval           time.Duration
	Count              int // number of samples
	WindSum            int // sum
	WindSum2           int // sum of squares
	WindMax            int
	WindMin            int
	WindDirXSum        float64 // sum
	WindDirYSum        float64 // sum
	WindDirMax         int
	WindDirMin         int
	BarometerSum       float64
	BarometerStart     float64
	OutsideTempSum     float64
	OutsideHumiditySum int
	BarTrendByte       byte
	Done               bool
}

func newRollup(period time.Time, interval time.Duration) *Rollup {
	return &Rollup{
		Period:     period,
		Interval:   interval,
		WindMin:    math.MaxInt32,
		WindDirMin: math.MaxInt32,
	}
}

func (r *Rollup) Update(loopRecord *vantage.LoopRecord) {
	r.Count++
	// wind
	wind := loopRecord.Wind
	r.WindSum += wind
	r.WindSum2 += wind * wind
	if wind > r.WindMax {
		r.WindMax = wind
	}
	if wind < r.WindMin {
		r.WindMin = wind
	}
	// wind direction
	winddir := loopRecord.WindDirection
	winddirx := math.Cos(float64(winddir) * RadiansPerDegree)
	winddiry := math.Sin(float64(winddir) * RadiansPerDegree)

	r.WindDirXSum += winddirx
	r.WindDirYSum += winddiry
	if winddir > r.WindDirMax {
		r.WindDirMax = winddir
	}
	if winddir < r.WindDirMin {
		r.WindDirMin = winddir
	}
	// other stuff
	r.BarometerSum += float64(loopRecord.Barometer())
	if r.Count == 1 {
		r.BarometerStart = float64(loopRecord.Barometer())
	}
	r.OutsideTempSum += float64(loopRecord.OutsideTemp())
	r.OutsideHumiditySum += loopRecord.OutsideHumidity
	r.BarTrendByte = loopRecord.BarTrendByte
}

func (r *Rollup) WindAvg() float64 {
	return float64(r.WindSum) / float64(r.Count)
}
func (r *Rollup) WindStddev() float64 {
	//variance = (SumSq - (Sum × Sum) ⁄ n) ⁄ n
	return math.Sqrt((float64(r.WindSum2) - float64(r.WindSum*r.WindSum)/float64(r.Count)) / float64(r.Count))
}
func (r *Rollup) WindDirAvg() int64 {
	rads := math.Atan2(r.WindDirYSum, r.WindDirXSum)
	if rads < 0 {
		rads += 2 * math.Pi
	}
	return int64(rads*DegreesPerRadian + 0.5)
}

func (r *Rollup) BarometerAvg() float64 {
	return r.BarometerSum / float64(r.Count)
}
func (r *Rollup) OutsideTempAvg() float64 {
	return r.OutsideTempSum / float64(r.Count)
}
func (r *Rollup) OutsideHumidityAvg() int {
	return r.OutsideHumiditySum / r.Count
}

func (r *Rollup) Summary() *Summary {
	s := &Summary{
		ID:                 0,
		StartTime:          r.Period,
		EndTime:            r.Period.Add(r.Interval),
		Measurements:       int64(r.Count),
		SummarySeconds:     int64(r.Interval / time.Second),
		WindAvg:            r.WindAvg(),
		WindGust:           float64(r.WindMax),
		WindLull:           float64(r.WindMin),
		WindStddev:         r.WindStddev(),
		WindDirectionAvg:   r.WindDirAvg(),
		WindDirectionMin:   int64(r.WindDirMin),
		WindDirectionMax:   int64(r.WindDirMax),
		BarometerAvg:       r.BarometerAvg(),
		BarometerStart:     r.BarometerStart,
		OutsideTempAvg:     r.OutsideTempAvg(),
		OutsideHumidityAvg: float64(r.OutsideHumidityAvg()),
		BarTrendByte:       r.BarTrendByte,
	}
	return s
}

func (s *Summary) insert() []interface{} {
	//(start_time,end_time,measurments,summary_seconds,wind_avg,wind_gust,wind_lull,wind_stddev,
	//wind_direction_avg,wind_direction_min,wind_direction_max,barometer_avg,barometer_start,outside_temp_avg,outside_humidity_avg)
	vals := make([]interface{}, len(insertCols))
	vals[0] = s.StartTime
	vals[1] = s.EndTime
	vals[2] = s.Measurements
	vals[3] = s.SummarySeconds
	vals[4] = s.WindAvg
	vals[5] = s.WindGust
	vals[6] = s.WindLull
	vals[7] = s.WindStddev
	vals[8] = s.WindDirectionAvg
	vals[9] = s.WindDirectionMin
	vals[10] = s.WindDirectionMax
	vals[11] = s.BarometerAvg
	vals[12] = s.BarometerStart
	vals[13] = s.OutsideTempAvg
	vals[14] = s.OutsideHumidityAvg
	return vals
}

func NewMysql(user, password string) (*Mysql, error) {
	connectString := fmt.Sprintf("%v:%v@/windygo?parseTime=true", user, password)
	gormDB, err := gorm.Open("mysql", connectString)
	//db, err := sql.Open("mysql", connectString)
	if err != nil {
		return nil, fmt.Errorf("error connecting to mysql: %w", err)
	}

	mysql := &Mysql{
		DB:  gormDB.DB(),
		ORM: gormDB,
	}
	mysql.rollups = make([]*Rollup, len(Intervals))
	mysql.SavedChan = make(chan *Summary, 10)
	mysql.ErrChan = make(chan error, 1)
	if err = mysql.init(); err != nil {
		return nil, err
	}
	mysql.insertStmt, err = mysql.DB.Prepare(mysql.createInsertStmt())
	if err != nil {
		return nil, fmt.Errorf("failed insert prepare: %w", err)
	}
	return mysql, nil
}

func (m *Mysql) createInsertStmt() string {
	placeholders := make([]string, len(insertCols))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return fmt.Sprintf(insertSql, strings.Join(insertCols, ","), strings.Join(placeholders, ","))
}

func (m *Mysql) init() error {
	_, err := m.DB.Exec(summariesTable)
	if err != nil {
		return fmt.Errorf("create summaries table error: %w", err)
	}
	m.ORM.AutoMigrate(&LoopRecord{})
	return nil
}

func (m *Mysql) Record(loopPkt []byte) {
	loopRecord := vantage.ParseLoop(loopPkt)
	finished := make([]*Rollup, 0, len(Intervals))
	for idx, interval := range Intervals {
		tint := loopRecord.Recorded.Truncate(interval)
		rollup := m.rollups[idx]
		if rollup == nil {
			rollup = newRollup(tint, interval)
			m.rollups[idx] = rollup
		}
		// the current loop record is after the rollup period
		// save the old rollup as finished and create a new one
		if !rollup.Period.Equal(tint) {
			rollup.Done = true
			finished = append(finished, rollup)
			rollup = newRollup(tint, interval)
			m.rollups[idx] = rollup
		}
		rollup.Update(loopRecord)
	}
	for _, done := range finished {
		m.save(done)
	}
}

func (m *Mysql) save(rollup *Rollup) {
	if rollup.Count == 0 {
		return
	}
	s := rollup.Summary()
	_, err := m.insertStmt.Exec(s.insert()...)
	if err != nil {
		select {
		case m.ErrChan <- fmt.Errorf("insert err: %w", err):
		default:
			log.Printf("Insert err: %v\n", err)
		}
	}
	select {
	case m.SavedChan <- s:
	default:
	}
}

// 5 minutes
const selectRecent string = "select * from summaries where end_time > ? and summary_seconds = ? order by end_time limit ?"

func (m *Mysql) GetSummaries(startTime time.Time, reportSize time.Duration, summarySecondsForReport int) ([]*Summary, error) {
	slenmin := int(reportSize / (time.Duration(summarySecondsForReport) * time.Second))
	var ss []*Summary
	err := m.ORM.Raw(selectRecent, startTime, summarySecondsForReport, slenmin).Find(&ss).Error
	//rows, err := m.DB.Query(selectRecent, startTime, summarySecondsForReport, slenmin)
	if err != nil {
		return nil, fmt.Errorf("failed to select summaries: %w", err)
	}
	/*
		ss := make([]*Summary, slenmin)
		i := 0
		for rows.Next() {
			if i == slenmin {
				rows.Close()
				log.Printf("Got too many summary records from DB")
				break
			}
			s := &Summary{}
			err := rows.Scan(
				&s.ID,
				&s.StartTime,
				&s.EndTime,
				&s.Measurements,
				&s.SummarySeconds,
				&s.WindAvg,
				&s.WindGust,
				&s.WindLull,
				&s.WindStddev,
				&s.WindDirectionAvg,
				&s.WindDirectionMin,
				&s.WindDirectionMax,
				&s.BarometerAvg,
				&s.BarometerStart,
				&s.OutsideTempAvg,
				&s.OutsideHumidityAvg)
			if err != nil {
				rows.Close()
				return nil, fmt.Errorf("error scanning summaries: %w", err)
			}
			// set times to local
			s.StartTime = s.StartTime.In(time.Local)
			s.EndTime = s.EndTime.In(time.Local)
			ss[i] = s
			i++
		}
	*/
	// warn if we have less than half the records
	i := len(ss)
	if i < (slenmin / 2) {
		log.Printf("Not enough summary records for report: %v/%v", i, slenmin)
	}
	ret := make([]*Summary, slenmin)
	for i, s := range ss {
		s.StartTime = s.StartTime.In(time.Local)
		s.EndTime = s.EndTime.In(time.Local)
		ret[i] = s
	}
	return ret, nil
}
