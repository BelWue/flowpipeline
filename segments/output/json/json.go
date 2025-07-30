// Prints all flows to stdout or a given file in json format, for consumption by the stdin segment or for debugging.
package json

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/klauspost/compress/zstd"
	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/segments"
	"google.golang.org/protobuf/encoding/protojson"
)

type Json struct {
	segments.BaseSegment
	writer *bufio.Writer

	FileName string // optional, default is empty which means stdout
	Pretty   bool   // optional, default is false
}

func (segment Json) New(config map[string]string) segments.Segment {
	newsegment := &Json{}

	var filename string = "stdout"
	var file *os.File
	var err error
	if config["filename"] != "" {
		file, err = os.Create(config["filename"])
		if err != nil {
			log.Error().Err(err).Msg("Json: File specified in 'filename' is not accessible: ")
		}
		filename = config["filename"]
	} else {
		file = os.Stdout
		log.Info().Msg("Json: 'filename' unset, using stdout.")
	}
	// configure zstd compression
	if config["zstd"] != "" {
		rawLevel, err := strconv.Atoi(config["zstd"])
		var level zstd.EncoderLevel
		if err != nil {
			log.Warn().Err(err).Msg("Json: Unable to parse zstd option, using default: ")
			level = zstd.SpeedDefault
		} else {
			level = zstd.EncoderLevelFromZstd(rawLevel)
		}
		encoder, err := zstd.NewWriter(file, zstd.WithEncoderLevel(level))
		if err != nil {
			log.Fatal().Err(err).Msg("Json: error creating zstd encoder: ")
		}
		newsegment.writer = bufio.NewWriter(encoder)
	} else {
		// no compression
		newsegment.writer = bufio.NewWriter(file)
	}
	var pretty bool
	if config["pretty"] != "" {
		pretty, err = strconv.ParseBool(config["pretty"])
		if err != nil {
			log.Warn().Err(err).Msg("Json: Unable to parse 'pretty' option, using 'false'")
			pretty = false
		}
	} else {
		log.Info().Msg("Json: 'pretty' unset, using default 'false'.")
		pretty = false
	}

	newsegment.FileName = filename
	newsegment.Pretty = pretty

	return newsegment
}

func (segment *Json) Run(wg *sync.WaitGroup) {
	defer func() {
		_ = segment.writer.Flush()
		close(segment.Out)
		wg.Done()
	}()

	marshalOptions := protojson.MarshalOptions{Multiline: segment.Pretty}
	for msg := range segment.In {
		data, err := marshalOptions.Marshal(msg)
		if err != nil {
			log.Warn().Err(err).Msg("Json: Skipping a flow, failed to recode protobuf as JSON: ")
			continue
		}

		// use Fprintln because it adds an OS specific newline
		_, err = fmt.Fprintln(segment.writer, string(data))
		if err != nil {
			log.Warn().Err(err).Msgf("Json: Skipping a flow, failed to write to file %s", segment.FileName)
			continue
		}
		// we need to flush here every time because we need full lines and can not wait
		// in case of using this output as in input for other instances consuming flow data
		_ = segment.writer.Flush()
		segment.Out <- msg
	}
}

func init() {
	segment := &Json{}
	segments.RegisterSegment("json", segment)
}
