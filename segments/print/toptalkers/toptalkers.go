package toptalkers

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/segments"
	"github.com/asecurityteam/rolling"
	"github.com/dustin/go-humanize"
)

type Record struct {
	DstIp   string
	Bytes   *rolling.TimePolicy
	Packets *rolling.TimePolicy
}

type TopTalkers struct {
	segments.BaseSegment
	writer *bufio.Writer

	Window         int    // optional, default is 60, sets the number of seconds used as a sliding window size
	ReportInterval int    // optional, default is 10, sets the number of seconds between report printing
	FileName       string // optional, default is "" (stdout), where to put the toptalker logs
	LogPrefix      string // optional, default is "", a prefix for each log line, useful in case multiple segments log to the same file
	ThresholdBps   uint64 // optional, default is 0, only log talkers with an average bits per second rate higher than this value
	ThresholdPps   uint64 // optional, default is 0, only log talkers with an average packets per second rate higher than this value
	TopN           uint64 // optional, default is 10, sets the number of top talkers per report
}

func (segment TopTalkers) New(config map[string]string) segments.Segment {
	newsegment := &TopTalkers{
		Window:         60,
		ReportInterval: 10,
		FileName:       config["filename"],
		LogPrefix:      config["logprefix"],
		TopN:           10,
	}

	if config["window"] != "" {
		if parsedWindow, err := strconv.ParseInt(config["window"], 10, 64); err == nil {
			if parsedWindow > math.MaxInt {
				log.Error().Msgf("TopTalkers: Window has to be <= %d.", math.MaxInt)
				return nil
			}
			if parsedWindow <= 0 {
				log.Error().Msg("TopTalkers: Window has to be >0.")
				return nil
			}
			newsegment.Window = int(parsedWindow)
		} else {
			log.Error().Msg("TopTalkers: Could not parse 'window' parameter, using default 60.")
		}
	} else {
		log.Info().Msg("TopTalkers: 'window' set to default 60.")
	}

	if config["reportinterval"] != "" {
		if parsedReportInterval, err := strconv.ParseInt(config["reportinterval"], 10, 64); err == nil {
			if parsedReportInterval > math.MaxInt {
				log.Error().Msgf("TopTalkers: Reportinterval has to be <= %d.", math.MaxInt)
				return nil
			}
			newsegment.ReportInterval = int(parsedReportInterval)
			if newsegment.ReportInterval <= 0 {
				log.Error().Msg("TopTalkers: Reportinterval has to be >0.")
				return nil
			}
		} else {
			log.Error().Msg("TopTalkers: Could not parse 'reportinterval' parameter, using default 60.")
		}
	} else {
		log.Info().Msg("TopTalkers: 'reportinterval' set to default 60.")
	}

	if config["filename"] != "" {
		file, err := os.Create(config["filename"])
		if err != nil {
			log.Error().Err(err).Msg("TopTalkers: File specified in 'filename' is not accessible: ")
		}
		newsegment.FileName = config["filename"]
		newsegment.writer = bufio.NewWriter(file)
	} else {
		newsegment.writer = bufio.NewWriter(os.Stdout)
		log.Info().Msg("TopTalkers: Parameter 'filename' empty, output goes to StdOut by default.")
	}

	if config["thresholdbps"] != "" {
		if parsedThresholdBps, err := strconv.ParseUint(config["thresholdbps"], 10, 32); err == nil {
			newsegment.ThresholdBps = parsedThresholdBps
		} else {
			log.Error().Msg("TopTalkers: Could not parse 'thresholdbps' parameter, using default 0.")
		}
	} else {
		log.Info().Msg("TopTalkers: 'thresholdbps' set to default '0'.")
	}

	if config["thresholdpps"] != "" {
		if parsedThresholdPps, err := strconv.ParseUint(config["thresholdpps"], 10, 32); err == nil {
			newsegment.ThresholdPps = parsedThresholdPps
		} else {
			log.Error().Msg("TopTalkers: Could not parse 'thresholdpps' parameter, using default 0.")
		}
	} else {
		log.Info().Msg("TopTalkers: 'thresholdpps' set to default '0'.")
	}

	if config["topn"] != "" {
		if parsedTopN, err := strconv.ParseUint(config["topn"], 10, 64); err == nil {
			newsegment.TopN = parsedTopN
			if newsegment.TopN <= 0 {
				log.Error().Msg("TopTalkers: TopN has to be >0.")
				return nil
			}
		} else {
			log.Error().Msg("TopTalkers: Could not parse 'topn' parameter, using default 10.")
		}
	} else {
		log.Info().Msg("TopTalkers: 'topn' set to default 10.")
	}

	return newsegment
}

func (segment *TopTalkers) Run(wg *sync.WaitGroup) {
	defer func() {
		segment.writer.Flush()
		close(segment.Out)
		wg.Done()
	}()
	database := map[string]*Record{}

	ticker := time.NewTicker(time.Duration(segment.ReportInterval) * time.Second)

	for {
		select {
		case <-ticker.C:
			databaseEntries := []*Record{}
			for _, entry := range database {
				databaseEntries = append(databaseEntries, entry)
			}
			sort.Slice(databaseEntries, func(i, j int) bool {
				iBytes := databaseEntries[i].Bytes.Reduce(rolling.Sum)
				if iBytes == 0 {
					delete(database, databaseEntries[i].DstIp)
				}
				jBytes := databaseEntries[j].Bytes.Reduce(rolling.Sum)
				if jBytes == 0 {
					delete(database, databaseEntries[j].DstIp)
				}
				return iBytes > jBytes
			})
			var printedRecords uint64 = 0
			fmt.Fprintln(segment.writer, segment.LogPrefix+"===================================================================")
			for _, record := range databaseEntries {
				bps := record.Bytes.Reduce(rolling.Sum) * 8 / float64(segment.Window)
				pps := record.Packets.Reduce(rolling.Sum) * 8 / float64(segment.Window)
				if bps < float64(segment.ThresholdBps) || pps < float64(segment.ThresholdPps) {
					break
				}
				fmt.Fprintf(segment.writer, "%s%s: %s, %s\n",
					segment.LogPrefix,
					record.DstIp,
					humanize.SI(bps, "bps"),
					humanize.SI(pps, "pps"),
				)
				printedRecords += 1
				if printedRecords >= segment.TopN {
					break
				}
			}
			segment.writer.Flush()
		case msg, ok := <-segment.In:
			if !ok {
				return
			}
			record := database[msg.DstAddrObj().String()]
			if record == nil {
				record = &Record{
					DstIp:   msg.DstAddrObj().String(),
					Bytes:   rolling.NewTimePolicy(rolling.NewWindow(segment.Window), time.Second),
					Packets: rolling.NewTimePolicy(rolling.NewWindow(segment.Window), time.Second),
				}
			}
			record.Bytes.Append(float64(msg.Bytes))
			record.Packets.Append(float64(msg.Packets))
			database[msg.DstAddrObj().String()] = record

			segment.Out <- msg
		}
	}
}

func init() {
	segment := &TopTalkers{}
	segments.RegisterSegment("toptalkers", segment)
}
