// The flowpipeline utility unifies all bwNetFlow functionality and
// provides configurable pipelines to process flows in any manner.
//
// The main entrypoint accepts command line flags to point to a configuration
// file and to establish the log level.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"plugin"
	"runtime"
	"strings"

	"github.com/hashicorp/logutils"

	"github.com/bwNetFlow/flowpipeline/pipeline"
	_ "github.com/bwNetFlow/flowpipeline/segments/alert/http"
	_ "github.com/bwNetFlow/flowpipeline/segments/analysis/toptalkers_metrics"
	_ "github.com/bwNetFlow/flowpipeline/segments/controlflow/branch"
	_ "github.com/bwNetFlow/flowpipeline/segments/export/clickhouse"
	_ "github.com/bwNetFlow/flowpipeline/segments/export/influx"
	_ "github.com/bwNetFlow/flowpipeline/segments/export/prometheus"
	_ "github.com/bwNetFlow/flowpipeline/segments/filter/drop"
	_ "github.com/bwNetFlow/flowpipeline/segments/filter/elephant"
	_ "github.com/bwNetFlow/flowpipeline/segments/filter/flowfilter"
	_ "github.com/bwNetFlow/flowpipeline/segments/input/bpf"
	_ "github.com/bwNetFlow/flowpipeline/segments/input/goflow"
	_ "github.com/bwNetFlow/flowpipeline/segments/input/kafkaconsumer"
	_ "github.com/bwNetFlow/flowpipeline/segments/input/packet"
	_ "github.com/bwNetFlow/flowpipeline/segments/input/stdin"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/addcid"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/addrstrings"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/anonymize"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/aslookup"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/bgp"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/dropfields"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/geolocation"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/normalize"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/protomap"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/remoteaddress"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/reversedns"
	_ "github.com/bwNetFlow/flowpipeline/segments/modify/snmp"
	_ "github.com/bwNetFlow/flowpipeline/segments/output/csv"
	_ "github.com/bwNetFlow/flowpipeline/segments/output/json"
	_ "github.com/bwNetFlow/flowpipeline/segments/output/kafkaproducer"
	_ "github.com/bwNetFlow/flowpipeline/segments/output/lumberjack"
	_ "github.com/bwNetFlow/flowpipeline/segments/output/sqlite"
	_ "github.com/bwNetFlow/flowpipeline/segments/pass"
	_ "github.com/bwNetFlow/flowpipeline/segments/print/count"
	_ "github.com/bwNetFlow/flowpipeline/segments/print/printdots"
	_ "github.com/bwNetFlow/flowpipeline/segments/print/printflowdump"
	_ "github.com/bwNetFlow/flowpipeline/segments/print/toptalkers"
)

var Version string

type flagArray []string

func (i *flagArray) String() string {
	return strings.Join(*i, ",")
}

func (i *flagArray) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	var pluginPaths flagArray
	flag.Var(&pluginPaths, "p", "Path to load segment plugins from, can be specified multiple times.")
	loglevel := flag.String("l", "warning", "Loglevel: one of 'debug', 'info', 'warning' or 'error'.")
	version := flag.Bool("v", false, "print version")
	concurrency := flag.Uint("j", 1, "How many concurrent pipelines to spawn. Set to 0 to enable automatic setting according to GOMAXPROCS. Only the default value 1 guarantees a stable order of the flows in and out of flowpipeline.")
	configfile := flag.String("c", "config.yml", "Location of the config file in yml format.")
	flag.Parse()

	if *version {
		fmt.Println(Version)
		return
	}

	log.SetOutput(&logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"debug", "info", "warning", "error"},
		MinLevel: logutils.LogLevel(*loglevel),
		Writer:   os.Stderr,
	})

	for _, path := range pluginPaths {
		_, err := plugin.Open(path)
		if err != nil {
			if err.Error() == "plugin: not implemented" {
				log.Println("[error] Loading plugins is unsupported when running a static, not CGO-enabled binary.")
			} else {
				log.Printf("[error] Problem loading the specified plugin: %s", err)
			}
			return
		} else {
			log.Printf("[info] Loaded plugin: %s", path)
		}
	}

	config, err := os.ReadFile(*configfile)
	if err != nil {
		log.Printf("[error] reading config file: %s", err)
		return
	}

	pipelineCount := 1
	if *concurrency == 0 {
		pipelineCount = runtime.GOMAXPROCS(0)
	} else {
		pipelineCount = int(*concurrency)
	}

	segmentReprs := pipeline.SegmentReprsFromConfig(config)
	for i := 0; i < pipelineCount; i++ {
		segments := pipeline.SegmentsFromRepr(segmentReprs)
		pipe := pipeline.New(segments...)
		pipe.Start()
		pipe.AutoDrain()
		defer pipe.Close()
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	<-sigs
}
