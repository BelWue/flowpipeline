package toptalkers_metrics

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Record struct {
	FwdBytes       []uint64
	FwdPackets     []uint64
	DropBytes      []uint64
	DropPackets    []uint64
	capacity       int
	pointer        int
	aboveThreshold atomic.Bool
	Address        string
	sync.RWMutex
}

type Database struct {
	database           *map[string]*Record
	TrafficType        string
	thresholdBps       uint64
	thresholdPps       uint64
	buckets            int
	BucketDuration     int // seconds
	ReportBuckets      int
	thresholdBuckets   int
	cleanupCounter     int
	cleanupWindowSizes int
	promExporter       *PrometheusExporter
	stopCleanupC       chan struct{}
	stopClockC         chan struct{}
	sync.RWMutex
}

func NewDatabase(params PrometheusMetricsParams, promExporter *PrometheusExporter) Database {
	return Database{
		database:           &map[string]*Record{},
		thresholdBps:       params.ThresholdBps,
		thresholdPps:       params.ThresholdPps,
		thresholdBuckets:   params.ThresholdBuckets,
		cleanupWindowSizes: params.CleanupWindowSizes,
		cleanupCounter:     params.Buckets * params.CleanupWindowSizes, // cleanup every N windows
		promExporter:       promExporter,
		buckets:            params.Buckets,
		ReportBuckets:      params.ReportBuckets,
		TrafficType:        params.TrafficType,
		BucketDuration:     params.BucketDuration,
		stopCleanupC:       make(chan struct{}),
		stopClockC:         make(chan struct{}),
	}
}

func (db *Database) GetRecord(address string) *Record {
	return db.GetTypedRecord("", address)
}

func (db *Database) GetTypedRecord(typeLabel string, address string) *Record {
	key := fmt.Sprintf("%s: %s", typeLabel, address)
	db.Lock()
	defer db.Unlock()
	record, found := (*db.database)[key]
	if !found || record == nil {
		record = NewRecord(db.ReportBuckets, address)
		(*db.database)[key] = record
	}
	return record
}

func NewRecord(windowSize int, address string) *Record {
	record := &Record{
		FwdBytes:    make([]uint64, windowSize),
		FwdPackets:  make([]uint64, windowSize),
		DropBytes:   make([]uint64, windowSize),
		DropPackets: make([]uint64, windowSize),
		capacity:    windowSize,
		pointer:     0,
		Address:     address,
	}
	return record
}

func (record *Record) Append(bytes uint64, packets uint64, statusFwd bool) {
	record.Lock()
	defer record.Unlock()
	if statusFwd {
		record.FwdBytes[record.pointer] += bytes
		record.FwdPackets[record.pointer] += packets
	} else {
		record.DropBytes[record.pointer] += bytes
		record.DropPackets[record.pointer] += packets
	}
}

func (record *Record) isEmpty() bool {
	record.RLock()
	defer record.RUnlock()
	for i := 0; i < record.capacity; i++ {
		if record.FwdPackets[i] > 0 || record.DropPackets[i] > 0 {
			return false
		}
	}
	return true
}

func (record *Record) GetMetrics(buckets int, bucketDuration int) (float64, float64, float64, float64, string) {
	// buckets == 0 means "look at the whole window"
	if buckets == 0 {
		buckets = record.capacity
	}
	sumFwdBytes := uint64(0)
	sumFwdPackets := uint64(0)
	sumDropBytes := uint64(0)
	sumDropPackets := uint64(0)
	record.RLock()
	defer record.RUnlock()
	pos := record.pointer
	for i := 0; i < buckets; i++ {
		if pos <= 0 {
			pos = record.capacity - 1
		} else {
			pos--
		}
		sumFwdBytes += record.FwdBytes[pos]
		sumFwdPackets += record.FwdPackets[pos]
		sumDropBytes += record.DropBytes[pos]
		sumDropPackets += record.DropPackets[pos]
	}
	sumFwdBps := float64(sumFwdBytes*8) / float64(buckets*bucketDuration)
	sumFwdPps := float64(sumFwdPackets) / float64(buckets*bucketDuration)
	sumDropBps := float64(sumDropBytes*8) / float64(buckets*bucketDuration)
	sumDropPps := float64(sumDropPackets) / float64(buckets*bucketDuration)
	return sumFwdBps, sumFwdPps, sumDropBps, sumDropPps, record.Address
}

func (record *Record) tick(thresholdBuckets int, bucketDuration int, thresholdBps uint64, thresholdPps uint64) {
	record.Lock()
	defer record.Unlock()
	// advance pointer to the next position
	record.pointer++
	if record.pointer >= record.capacity {
		record.pointer = 0
	}
	// calculate averages and check thresholds
	if thresholdBuckets == 0 {
		// thresholdBuckets == 0 means "look at the whole window"
		thresholdBuckets = record.capacity
	}
	var sumBytes uint64
	var sumPackets uint64
	pos := record.pointer
	for i := 0; i < thresholdBuckets; i++ {
		if pos <= 0 {
			pos = record.capacity - 1
		} else {
			pos--
		}
		sumBytes = sumBytes + record.FwdBytes[pos] + record.DropBytes[pos]
		sumPackets = sumPackets + record.FwdPackets[pos] + record.DropPackets[pos]
	}
	bps := uint64(float64(sumBytes*8) / float64(bucketDuration*thresholdBuckets))
	pps := uint64(float64(sumPackets) / float64(bucketDuration*thresholdBuckets))
	if (bps > thresholdBps) && (pps > thresholdPps) {
		record.aboveThreshold.Store(true)
	} else {
		record.aboveThreshold.Store(false)
	}
	// clear the current bucket
	record.FwdBytes[record.pointer] = 0
	record.FwdPackets[record.pointer] = 0
	record.DropBytes[record.pointer] = 0
	record.DropPackets[record.pointer] = 0
}

func (db *Database) Clock() {
	ticker := time.NewTicker(time.Duration(db.BucketDuration) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			db.Lock()
			for _, record := range *db.database {
				record.tick(db.thresholdBuckets, db.BucketDuration, db.thresholdBps, db.thresholdPps)
			}
			db.Unlock()
		case <-db.stopClockC:
			return
		}
	}
}

func (db *Database) Cleanup() {
	ticker := time.NewTicker(time.Duration(db.BucketDuration*db.buckets) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			db.Lock()
			db.cleanupCounter--
			if db.cleanupCounter <= 0 {
				db.cleanupCounter = db.buckets * db.cleanupWindowSizes
				for key, record := range *db.database {
					if record.isEmpty() {
						delete(*db.database, key)
					}
				}
			}
			db.promExporter.dbSize.Set(float64(len(*db.database)))
			db.Unlock()
		case <-db.stopCleanupC:
			return
		}
	}
}

func (db *Database) StopTimers() {
	var stopmessage struct{}
	db.stopClockC <- stopmessage
}

func (db *Database) GetAllRecords() <-chan struct {
	key    string
	record *Record
} {
	out := make(chan struct {
		key    string
		record *Record
	})
	go func() {
		db.Lock()
		defer func() {
			db.Unlock()
			close(out)
		}()
		for key, record := range *db.database {
			out <- struct {
				key    string
				record *Record
			}{key, record}
		}
	}()
	return out
}
