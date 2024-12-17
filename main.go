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
	"strings"

	"github.com/bwNetFlow/flowpipeline/pipeline"
	"github.com/hashicorp/logutils"

	_ "github.com/bwNetFlow/flowpipeline/segments/alert/http"

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

	_ "github.com/bwNetFlow/flowpipeline/segments/pass"

	_ "github.com/bwNetFlow/flowpipeline/segments/output/csv"
	_ "github.com/bwNetFlow/flowpipeline/segments/output/json"
	_ "github.com/bwNetFlow/flowpipeline/segments/output/kafkaproducer"
	_ "github.com/bwNetFlow/flowpipeline/segments/output/lumberjack"
	_ "github.com/bwNetFlow/flowpipeline/segments/output/mongodb"
	_ "github.com/bwNetFlow/flowpipeline/segments/output/sqlite"

	_ "github.com/bwNetFlow/flowpipeline/segments/print/count"
	_ "github.com/bwNetFlow/flowpipeline/segments/print/printdots"
	_ "github.com/bwNetFlow/flowpipeline/segments/print/printflowdump"
	_ "github.com/bwNetFlow/flowpipeline/segments/print/toptalkers"

	_ "github.com/bwNetFlow/flowpipeline/segments/analysis/toptalkers_metrics"
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
	flag.Var(&pluginPaths, "p", "path to load segment plugins from, can be specified multiple times")
	loglevel := flag.String("l", "warning", "loglevel: one of 'debug', 'info', 'warning' or 'error'")
	version := flag.Bool("v", false, "print version")
	configfile := flag.String("c", "config.yml", "location of the config file in yml format")
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
	pipe := pipeline.NewFromConfig(config)
	pipe.Start()
	pipe.AutoDrain()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	<-sigs

	pipe.Close()
}
