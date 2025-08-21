// The `json` segment provides a JSON output option. It uses stdout by default, but can
// be instructed to write to file using the filename parameter. This is intended to be
// able to pipe flows between instances of flowpipeline, but it is also very useful when
// debugging flowpipelines or to create a quick plaintext dump.
//
// If the option `zstd` is set, the output will be compressed using the
// [zstandard algorithm](https://facebook.github.io/zstd/). If the option `zstd` is set
// to a positive integer, the compression level will be set to
// ([approximately](https://github.com/klauspost/compress/tree/master/zstd#status)) that
// value. When `flowpipeline` is stopped abruptly (e.g by pressing Ctrl+C), the end of
// the archive will get corrupted. Simply use `zstdcat` to decompress the archive and
// remove the last line (`| head -n -1`).
//
// If the option `pretty` is set to true, the every flow will be formatted in a
// human-readable way (indented and with line breaks). When omitted, the output will be
// a single line per flow.
package json

import (
	"bufio"
	"fmt"
	"strconv"
	"sync"

	"github.com/klauspost/compress/zstd"
	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/segments"
	"google.golang.org/protobuf/encoding/protojson"
)

type Json struct {
	segments.BaseTextOutputSegment
	writer *bufio.Writer
	Pretty bool // optional, default is false
}

func (segment Json) New(config map[string]string) segments.Segment {
	newsegment := &Json{}
	file, err := segment.GetOutput(config)
	if err != nil {
		log.Error().Err(err).Msg("Json: File specified in 'filename' is not accessible: ")
		return nil
	}
	log.Info().Msgf("Json: configured output to %s", file.Name())

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

	newsegment.Pretty = pretty

	return newsegment
}

func (segment *Json) Run(wg *sync.WaitGroup) {
	defer func() {
		_ = segment.writer.Flush()
		segment.File.Close()
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
			log.Warn().Err(err).Msgf("Json: Skipping a flow, failed to write to file %s", segment.File.Name())
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
