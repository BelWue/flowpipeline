// The `lumberjack` segment sends flows to one or more [elastic beats](https://github.com/elastic/beats)
// servers user the [lumberjack](https://github.com/logstash-plugins/logstash-input-beats/blob/main/PROTOCOL.md)
// protocol. Flows are queued in a non-deterministic, round-robin fashion to the servers.
//
// The only mandatory option is `servers` which contains a comma-separated list of lumberjack
// server URLs. Each URL must start with one of these schemata: `tcp://` (plain TCP,
// no encryption), `tls://` (TLS encryption) or `tlsnoverify://` (TLS encryption without
// certificate verification). The schema is followed by the hostname or IP address, a colon `:`,
// and a port number. IPv6 addresses must be surrounded by square brackets.
//
// A goroutine is spawned for every lumberjack server. Each goroutine only uses one CPU core to
// process and send flows. This may not be enough when the ingress flow rate is high and/or a high compression
// level is used. The number of goroutines per backend can be set explicitly with the `?count=x` URL
// parameter. For example:
//
// ```yaml
// config:
//
//	server: tls://host1:5043/?count=4, tls://host2:5043/?compression=9&count=16
//
// ```
//
// will use four parallel goroutines for `host1` and sixteen parallel goroutines for `host2`. Use `&count=…` instead of
// `?count=…` when `count` is not the first parameter (standard URI convention).
//
// Transport compression is disabled by default. Use `compression` to set the compression level
// for all hosts. Compression levels can vary between 0 (no compression) and 9 (maximum compression).
// To set per-host transport compression adding `?compression=<level>` to the server URI.
//
// To prevent blocking, flows are buffered in a channel between the segment and the output
// go routines. Each output go routine maintains a buffer of flows which are send either when the
// buffer is full or after a configurable timeout. Proper parameter sizing for the queue,
// buffers, and timeouts depends on multiple individual factors (like size, characteristics
// of the incoming netflows and the responsiveness of the target servers). There are parameters
// to both observe and tune this segment's performance.
//
// Upon connection error or loss, the segment will try to reconnect indefinitely with a pause of
// `reconnectwait` between attempts.
//
// * `queuesize` (integer) sets the number of flows that are buffered between the segment and the output go routines.
// * `batchsize` (integer) sets the number of flows that each output go routine buffers before sending.
// * `batchtimeout` (duration) sets the maximum time that flows are buffered before sending.
// * `reconnectwait` (duration) sets the time to wait between reconnection attempts.
//
// These options help to observe the performance characteristics of the segment:
//
// * `batchdebug` (bool) enables debug logging of batch operations (full send, partial send, and skipped send).
// * `queuestatusinterval` (duration) sets the interval at which the segment logs the current queue status.
//
// To see debug output, set the `-l debug` flag when starting `flowpipeline`.
//
// See [time.ParseDuration](https://pkg.go.dev/time#ParseDuration) for proper duration format
// strings and [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool) for allowed bool keywords.
//
// ```yaml
//   - segment: lumberjack
//     config:
//     servers: tcp://foo.example.com:5044, tls://bar.example.com:5044?compression=3, tlsnoverify://[2001:db8::1]:5044
//     compression: 0
//     batchsize: 1024
//     queuesize: = 2048
//     batchtimeout: "2000ms"
//     reconnectwait: "1s"
//     batchdebug: false
//     queuestatusinterval: "0s"
//
// ```
//
// [godoc](https://pkg.go.dev/github.com/BelWue/flowpipeline/segments/output/lumberjack)
package lumberjack

import (
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
)

const (
	defaultQueueSize           = 65536
	defaultBatchSize           = 64
	defaultTimeout             = 5 * time.Second
	minimalBatchTimeout        = 50 * time.Millisecond
	defaultQueueStatusInterval = 0 * time.Second
	defaultReconnectWait       = 1 * time.Second
)

type ServerOptions struct {
	UseTLS            bool
	VerifyCertificate bool
	CompressionLevel  int
	Parallelism       int
}

type Lumberjack struct {
	segments.BaseSegment
	Servers             map[string]ServerOptions
	BatchSize           int
	BatchTimeout        time.Duration
	BatchDebugPrintf    func(format string, v ...any)
	QueueStatusInterval time.Duration
	ReconnectWait       time.Duration
	LumberjackOut       chan *pb.EnrichedFlow
}

func NoDebugPrintf(format string, v ...any) { _, _ = format, v }
func DoDebugPrintf(format string, v ...any) {
	log.Debug().Msgf(format, v...)
}

func (segment *Lumberjack) New(config map[string]string) segments.Segment {
	var (
		err                error
		bufferLength       int
		defaultCompression int
	)

	// parse default compression level
	defaultCompressionString := config["compression"]
	if defaultCompressionString == "" {
		defaultCompression = 0
	} else {
		defaultCompression, err = strconv.Atoi(defaultCompressionString)
		if err != nil {
			log.Fatal().Err(err).Msgf("Lumberjack: Failed to parse default compression level %s", defaultCompressionString)
		}
		if defaultCompression < 0 || defaultCompression > 9 {
			log.Fatal().Msgf("Lumberjack: Default compression level %d is out of range", defaultCompression)
		}
	}

	// parse server URLs
	rawServerStrings := strings.Split(config["servers"], ",")
	for idx, serverName := range rawServerStrings {
		rawServerStrings[idx] = strings.TrimSpace(serverName)
	}
	if len(rawServerStrings) == 0 {
		log.Fatal().Msg("Lumberjack: No servers specified in 'servers' config option.")
	} else {
		segment.Servers = make(map[string]ServerOptions)
		for _, rawServerString := range rawServerStrings {
			serverURL, err := url.Parse(rawServerString)
			if err != nil {
				log.Fatal().Err(err).Msgf("Lumberjack: Failed to parse server URL %s", rawServerString)
			}
			urlQueryParams := serverURL.Query()

			// parse TLS options
			var useTLS, verifyTLS bool
			switch serverURL.Scheme {
			case "tcp":
				useTLS = false
				verifyTLS = false
			case "tls":
				useTLS = true
				verifyTLS = true
			case "tlsnoverify":
				useTLS = true
				verifyTLS = false
			default:
				log.Fatal().Msgf("Lumberjack: Unknown scheme %s in server URL %s", serverURL.Scheme, rawServerString)
			}

			// parse compression level
			var compressionLevel int
			compressionString := urlQueryParams.Get("compression")

			if compressionString == "" {
				// use global default if not specified
				compressionLevel = defaultCompression
			} else {
				compressionLevel, err = strconv.Atoi(compressionString)
				if err != nil {
					log.Fatal().Err(err).Msgf("Lumberjack: Failed to parse compression level %s for host %s", compressionString, serverURL.Host)
				}
				if compressionLevel < 0 || compressionLevel > 9 {
					log.Fatal().Msgf("Lumberjack: Compression level %d out of range for host %s", compressionLevel, serverURL.Host)
				}
			}

			// parse count url argument
			var numRoutines int
			numRoutinesString := urlQueryParams.Get("count")
			if numRoutinesString == "" {
				numRoutines = 1
			} else {
				numRoutines, err = strconv.Atoi(numRoutinesString)
				switch {
				case err != nil:
					log.Fatal().Err(err).Msgf("Lumberjack: Failed to parse count %s for host %s", numRoutinesString, serverURL.Host)
				case numRoutines < 1:
					log.Warn().Msgf("Lumberjack: count is smaller than 1, setting to 1")
					numRoutines = 1
				case numRoutines > runtime.NumCPU():
					log.Warn().Msgf("Lumberjack: count is larger than runtime.NumCPU (%d). This will most likely hurt performance.", runtime.NumCPU())
				}
			}

			segment.Servers[serverURL.Host] = ServerOptions{
				UseTLS:            useTLS,
				VerifyCertificate: verifyTLS,
				CompressionLevel:  compressionLevel,
				Parallelism:       numRoutines,
			}
		}
	}

	// parse batchSize option
	segment.BatchSize = defaultBatchSize
	if config["batchsize"] != "" {
		segment.BatchSize, err = strconv.Atoi(strings.ReplaceAll(config["batchsize"], "_", ""))
		if err != nil {
			log.Fatal().Err(err).Msg("Lumberjack: Failed to parse batchsize config option: ")
		}
	}
	if segment.BatchSize < 0 {
		segment.BatchSize = defaultBatchSize
	}
	// parse batchtimeout option
	segment.BatchTimeout = defaultTimeout
	if config["batchtimeout"] != "" {
		segment.BatchTimeout, err = time.ParseDuration(config["batchtimeout"])
		if err != nil {
			log.Fatal().Err(err).Msg("Lumberjack: Failed to parse timeout config option: ")
		}
	}

	if segment.BatchTimeout < minimalBatchTimeout {
		log.Error().Msgf("Lumberjack: timeout %s too small, using default %s", segment.BatchTimeout.String(), defaultTimeout.String())
		segment.BatchTimeout = defaultTimeout
	}
	if segment.BatchTimeout > time.Minute {
		log.Error().Msgf("Lumberjack: timeout %s too large, using default %s", segment.BatchTimeout.String(), defaultTimeout)
		segment.BatchTimeout = defaultTimeout
	}
	// parse batchdebug option
	if config["batchdebug"] != "" {
		batchDebug, err := strconv.ParseBool(config["batchdebug"])
		if err != nil {
			log.Fatal().Err(err).Msg("Lumberjack: Failed to parse batchdebug config option: ")
		}
		// set the correct BatchDebugPrintf function
		if batchDebug {
			segment.BatchDebugPrintf = DoDebugPrintf
		} else {
			segment.BatchDebugPrintf = NoDebugPrintf
		}
	}

	// parse reconnectwait option
	segment.ReconnectWait = defaultReconnectWait
	if config["reconnectwait"] != "" {
		segment.ReconnectWait, err = time.ParseDuration(config["reconnectwait"])
		if err != nil {
			log.Fatal().Err(err).Msg("Lumberjack: Failed to parse reconnectwait config option: ")
		}
	}

	// parse queueStatusInterval option
	segment.QueueStatusInterval = defaultQueueStatusInterval
	if config["queuestatusinterval"] != "" {
		segment.QueueStatusInterval, err = time.ParseDuration(config["queuestatusinterval"])
		if err != nil {
			log.Fatal().Err(err).Msg("Lumberjack: Failed to parse queuestatussnterval config option: ")
		}
	}

	// create buffered channel
	if config["queuesize"] != "" {
		bufferLength, err = strconv.Atoi(strings.ReplaceAll(config["queuesize"], "_", ""))
		if err != nil {
			log.Fatal().Err(err).Msg("Lumberjack: Failed to parse queuesize config option: ")
		}
	} else {
		bufferLength = defaultQueueSize
	}
	if bufferLength < 64 {
		log.Error().Msgf("Lumberjack: queuesize too small, using default %d", defaultQueueSize)
		bufferLength = defaultQueueSize
	}
	segment.LumberjackOut = make(chan *pb.EnrichedFlow, bufferLength)

	return segment
}

func (segment *Lumberjack) Run(wg *sync.WaitGroup) {
	var writerWG sync.WaitGroup

	defer func() {
		close(segment.Out)
		writerWG.Wait()
		wg.Done()
		log.Info().Msg("Lumberjack: All writer functions have stopped, exiting…")
	}()

	// print queue status information
	if segment.QueueStatusInterval > 0 {
		go func() {
			length := cap(segment.LumberjackOut)
			for {
				time.Sleep(segment.QueueStatusInterval)
				fill := len(segment.LumberjackOut)
				log.Debug().Msgf("Lumberjack: Queue is %3.2f%% full (%d/%d)", float64(fill)/float64(length)*100, fill, length)
			}
		}()
	}

	// run a goroutine for each lumberjack server
	for server, options := range segment.Servers {
		writerWG.Add(1)
		options := options
		for i := 0; i < options.Parallelism; i++ {
			go func(server string, numServer int) {
				defer writerWG.Done()
				// connect to lumberjack server
				client := newResilientClient(server, options, segment.ReconnectWait)
				defer client.Close()
				log.Info().Msgf("Lumberjack: Connected to %s (TLS: %v, VerifyTLS: %v, Compression: %d, number %d/%d)", server, options.UseTLS, options.VerifyCertificate, options.CompressionLevel, numServer+1, options.Parallelism)

				flowInterface := make([]interface{}, segment.BatchSize)
				idx := 0

				// see https://stackoverflow.com/questions/66037676/go-reset-a-timer-newtimer-within-select-loop for timer mechanics
				timer := time.NewTimer(segment.BatchTimeout)
				timer.Stop()
				defer timer.Stop()
				var timerSet bool

				for {
					select {
					case flow, isOpen := <-segment.LumberjackOut:
						// exit on channel closing
						if !isOpen {
							// send local buffer
							count, err := client.SendNoRetry(flowInterface[:idx])
							if err != nil {
								log.Error().Err(err).Msgf("Lumberjack: Failed to send final flow batch upon exit to %s", server)
							} else {
								segment.BatchDebugPrintf("Lumberjack: %s Sent final batch (%d)", server, count)
							}
							wg.Done()
							return
						}

						// append flow to batch
						flowInterface[idx] = ECSFromEnrichedFlow(flow)
						idx++

						// send batch if full
						if idx == segment.BatchSize {
							// We got an event, and the timer was already set.
							// We need to stop the timer and drain the channel
							// so that we can safely reset it later.
							if timerSet {
								if !timer.Stop() {
									<-timer.C
								}
								timerSet = false
							}

							client.Send(flowInterface)
							segment.BatchDebugPrintf("Lumberjack: %s Sent full batch (%d)", server, segment.BatchSize)

							// reset idx
							idx = 0

							// If the timer was not set, or it was stopped before, it's safe to reset it.
							if !timerSet {
								timerSet = true
								timer.Reset(segment.BatchTimeout)
							}
						}
					case <-timer.C:
						// timer expired, send batch
						if idx > 0 {
							segment.BatchDebugPrintf("Lumberjack: %s Sending incomplete batch (%d/%d)", server, idx, segment.BatchSize)
							client.Send(flowInterface[:idx])
							idx = 0
						} else {
							segment.BatchDebugPrintf("Lumberjack: %s Timer expired with empty batch", server)
						}

						timer.Reset(segment.BatchTimeout)
						timerSet = true
					}
				}
			}(server, i)
		}
	}

	// forward flows to lumberjack servers and to the next segment
	for msg := range segment.In {
		segment.LumberjackOut <- msg
		segment.Out <- msg
	}
	close(segment.LumberjackOut)
}

// register segment
func init() {
	segment := &Lumberjack{}
	segments.RegisterSegment("lumberjack", segment)
}
