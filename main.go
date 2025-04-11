// The flowpipeline utility unifies all bwNetFlow functionality and
// provides configurable pipelines to process flows in any manner.
//
// The main entrypoint accepts command line flags to point to a configuration
// file and to establish the log level.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"plugin"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/pipeline"

	_ "github.com/BelWue/flowpipeline/segments/alert/http"

	_ "github.com/BelWue/flowpipeline/segments/controlflow/branch"

	_ "github.com/BelWue/flowpipeline/segments/export/clickhouse"
	_ "github.com/BelWue/flowpipeline/segments/export/influx"
	_ "github.com/BelWue/flowpipeline/segments/export/prometheus"

	_ "github.com/BelWue/flowpipeline/segments/filter/drop"
	_ "github.com/BelWue/flowpipeline/segments/filter/elephant"

	_ "github.com/BelWue/flowpipeline/segments/filter/flowfilter"

	_ "github.com/BelWue/flowpipeline/segments/input/bpf"
	_ "github.com/BelWue/flowpipeline/segments/input/diskbuffer"
	_ "github.com/BelWue/flowpipeline/segments/input/goflow"
	_ "github.com/BelWue/flowpipeline/segments/input/kafkaconsumer"
	_ "github.com/BelWue/flowpipeline/segments/input/packet"
	_ "github.com/BelWue/flowpipeline/segments/input/stdin"

	_ "github.com/BelWue/flowpipeline/segments/modify/addcid"
	_ "github.com/BelWue/flowpipeline/segments/modify/addrstrings"
	_ "github.com/BelWue/flowpipeline/segments/modify/anonymize"
	_ "github.com/BelWue/flowpipeline/segments/modify/aslookup"
	_ "github.com/BelWue/flowpipeline/segments/modify/bgp"
	_ "github.com/BelWue/flowpipeline/segments/modify/dropfields"
	_ "github.com/BelWue/flowpipeline/segments/modify/geolocation"
	_ "github.com/BelWue/flowpipeline/segments/modify/normalize"
	_ "github.com/BelWue/flowpipeline/segments/modify/protomap"
	_ "github.com/BelWue/flowpipeline/segments/modify/remoteaddress"
	_ "github.com/BelWue/flowpipeline/segments/modify/reversedns"
	_ "github.com/BelWue/flowpipeline/segments/modify/snmp"

	_ "github.com/BelWue/flowpipeline/segments/pass"

	_ "github.com/BelWue/flowpipeline/segments/output/csv"
	_ "github.com/BelWue/flowpipeline/segments/output/json"
	_ "github.com/BelWue/flowpipeline/segments/output/kafkaproducer"
	_ "github.com/BelWue/flowpipeline/segments/output/lumberjack"
	_ "github.com/BelWue/flowpipeline/segments/output/mongodb"
	_ "github.com/BelWue/flowpipeline/segments/output/sqlite"

	_ "github.com/BelWue/flowpipeline/segments/print/count"
	_ "github.com/BelWue/flowpipeline/segments/print/printdots"
	_ "github.com/BelWue/flowpipeline/segments/print/printflowdump"
	_ "github.com/BelWue/flowpipeline/segments/print/toptalkers"

	_ "github.com/BelWue/flowpipeline/segments/analysis/toptalkers_metrics"
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
	logLevel := flag.String("l", "warning", "loglevel: one of 'trace', 'debug', 'info', 'warning', 'error', 'fatal', or 'panic'")
	version := flag.Bool("v", false, "print version")
	prettyLogging := flag.Bool("j", false, "Json log")
	configFile := flag.String("c", "config.yml", "location of the config file in yml format")
	flag.Parse()

	if *version {
		fmt.Println(Version)
		return
	}

	if !*prettyLogging {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.DateTime})
	}
	zerolog.SetGlobalLevel(zerologLogLevel(logLevel))
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	for _, path := range pluginPaths {
		_, err := plugin.Open(path)
		if err != nil {
			if err.Error() == "plugin: not implemented" {
				log.Error().Msg("Loading plugins is unsupported when running a static, not CGO-enabled binary.")
			} else {
				log.Error().Err(err).Msgf("Problem loading the specified plugin '%s'", path)
			}
			return
		} else {
			log.Info().Msgf("Loaded plugin: %s", path)
		}
	}

	config, err := os.ReadFile(*configFile)
	if err != nil {
		log.Error().Err(err).Msg(" reading config file: ")
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

func zerologLogLevel(logLevel *string) zerolog.Level {
	if logLevel != nil && *logLevel != "" {
		switch *logLevel {
		case "trace":
			log.Info().Msg("Using log level 'trace'")
			return zerolog.TraceLevel
		case "debug":
			log.Info().Msg("Using log level 'debug'")
			return zerolog.DebugLevel
		case "info":
			log.Info().Msg("Using log level 'info'")
			return zerolog.InfoLevel
		case "warning":
			return zerolog.WarnLevel
		case "error":
			return zerolog.ErrorLevel
		case "fatal":
			return zerolog.FatalLevel
		case "panic":
			return zerolog.PanicLevel
		default:
			log.Warn().Msgf("Unknown log level '%s' using default 'info'", *logLevel)
		}
	} else {
		log.Info().Msg("Using default log level 'info'")
	}

	return zerolog.InfoLevel
}
