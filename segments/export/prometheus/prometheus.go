// Collects and serves statistics about flows.
// Configuration options for this segments are:
// The endoint field can be configured:
// <host>:8080/flowdata
// <host>:8080/metrics
// The labels to be exported can be set in the configuration.
// Default labels are:
//
//	router,ipversion,application,protoname,direction,peer,remoteas,remotecountry
//
// Additional Labels are:
//
//	src_port,dst_port,src_ip,dst_ip
//
// The additional label should be used with care, because of infite quantity.
package prometheus

import (
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/go-co-op/gocron/v2"
)

type Prometheus struct {
	segments.BaseSegment
	Endpoint       string         // optional, default value is ":8080"
	MetricsPath    string         // optional, default is "/metrics"
	FlowdataPath   string         // optional, default is "/flowdata"
	Labels         []string       // optional, list of labels to be exported
	VacuumInterval *time.Duration // optional, intervall in which counters should be reset (can lead to dataloss)
}

func (segment Prometheus) New(config map[string]string) segments.Segment {
	var endpoint string = ":8080"
	if config["endpoint"] == "" {
		log.Info().Msg("prometheus: Missing configuration parameter 'endpoint'. Using default port \":8080\"")
	} else {
		endpoint = config["endpoint"]
	}
	var metricsPath string = "/metrics"
	if config["metricspath"] == "" {
		log.Info().Msg("prometheus: Missing configuration parameter 'metricspath'. Using default path \"/metrics\"")
	} else {
		metricsPath = config["metricspath"]
	}
	var flowdataPath string = "/flowdata"
	if config["flowdatapath"] == "" {
		log.Info().Msg("prometheus: Missing configuration parameter 'flowdatapath'. Using default path \"/flowdata\"")
	} else {
		flowdataPath = config["flowdatapath"]
	}
	var vacuumInterval *time.Duration
	if config["vacuum_interval"] != "" {
		vacuumIntervalDuration, err := time.ParseDuration(config["vacuum_interval"])
		if err != nil {
			log.Warn().Msg("prometheus: Failed to parse vacuum intervall \"" + config["vacuum_interval"] + "\" - continuing without vacuum interval")
		} else {
			log.Info().Msg("prometheus: Setting prometheus vacuum interval to " + config["vacuum_interval"] + " this will lead to data loss of up to one scraping intervall!")
			vacuumInterval = &vacuumIntervalDuration
		}
	}

	newsegment := &Prometheus{
		Endpoint:       endpoint,
		MetricsPath:    metricsPath,
		FlowdataPath:   flowdataPath,
		VacuumInterval: vacuumInterval,
	}

	// set default labels if not configured
	var labels []string
	if config["labels"] == "" {
		log.Info().Msg("prometheus: Configuration parameter 'labels' not set. Using default labels 'Etype,Proto' to export")
		labels = strings.Split("Etype,Proto", ",")
	} else {
		labels = strings.Split(config["labels"], ",")
	}
	protofields := reflect.TypeOf(pb.EnrichedFlow{})
	for _, field := range labels {
		field = strings.TrimSpace(field)
		_, found := protofields.FieldByName(field)
		if !found {
			log.Error().Msgf("Prometheus: Field '%s' specified in 'labels' does not exist.", field)
			return nil
		}
		newsegment.Labels = append(newsegment.Labels, field)
	}
	return newsegment
}

func (segment *Prometheus) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	var promExporter = Exporter{}
	segment.initializeExporter(&promExporter)
	if segment.VacuumInterval != nil {
		segment.AddVacuumCronJob(&promExporter)
	}

	for msg := range segment.In {
		labelset := make(map[string]string)
		values := reflect.ValueOf(msg).Elem()
		for _, fieldname := range segment.Labels {
			value := values.FieldByName(fieldname).Interface()
			switch value.(type) {
			case []uint8: // this is necessary for proper formatting
				ipstring := net.IP(value.([]uint8)).String()
				if ipstring == "<nil>" {
					ipstring = ""
				}
				labelset[fieldname] = ipstring
			case uint32: // this is because FormatUint is much faster than Sprint
				labelset[fieldname] = strconv.FormatUint(uint64(value.(uint32)), 10)
			case uint64: // this is because FormatUint is much faster than Sprint
				labelset[fieldname] = strconv.FormatUint(uint64(value.(uint64)), 10)
			case string: // this is because doing nothing is also much faster than Sprint
				labelset[fieldname] = value.(string)
			default:
				labelset[fieldname] = fmt.Sprint(value)
			}
		}
		promExporter.Increment(msg.Bytes, msg.Packets, labelset)
		segment.Out <- msg
	}
}

func (segment *Prometheus) initializeExporter(exporter *Exporter) {
	exporter.Initialize(segment.Labels)
	exporter.ServeEndpoints(segment)
}

func (segment *Prometheus) AddVacuumCronJob(promExporter *Exporter) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		log.Warn().Err(err).Msg("Failed initializing prometheus exporter vacuum job")
	}
	_, err = scheduler.NewJob(
		gocron.DurationJob(*segment.VacuumInterval),
		gocron.NewTask(promExporter.ResetCounter),
	)
	if err != nil {
		log.Warn().Err(err).Msg("Failed initializing prometheus exporter vacuum job")
	}
	// start the scheduler
	scheduler.Start()
}

func init() {
	segment := &Prometheus{}
	segments.RegisterSegment("prometheus", segment)
}
