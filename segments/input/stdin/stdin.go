// Receives flows from stdin in JSON format, as exported by the json segment.
// This segment can also read from a file with flows in json format per each line
package stdin

import (
	"bufio"
	"strconv"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"google.golang.org/protobuf/encoding/protojson"

	"os"
	"sync"
)

type StdIn struct {
	segments.BaseSegment
	scanner *bufio.Scanner

	FileName  string // optional, default is empty which means read from stdin
	EofCloses bool   // optional, default is false. Closes Pipeleine gracefully after input file was read
}

func (segment StdIn) New(config map[string]string) segments.Segment {
	newsegment := &StdIn{}

	var filename string = "stdout"
	var file *os.File
	var err error
	var eofCloses bool = false
	if config["filename"] != "" {
		file, err = os.Open(config["filename"])
		if err != nil {
			log.Error().Err(err).Msg("StdIn: File specified in 'filename' is not accessible: ")
			return nil
		}
		filename = config["filename"]
		if config["eofcloses"] != "" {
			if parsedClose, err := strconv.ParseBool(config["eofcloses"]); err == nil {
				eofCloses = parsedClose
			} else {
				log.Error().Msg("StdIn: Could not parse 'eofcloses' parameter, using default false.")
			}
		} else {
			log.Info().Msg("StdIn: 'eofcloses' set to default false.")
		}
	} else {
		file = os.Stdin
		log.Info().Msg("StdIn: 'filename' unset, using stdIn.")
	}
	newsegment.scanner = bufio.NewScanner(file)

	newsegment.FileName = filename
	newsegment.EofCloses = eofCloses

	return newsegment
}

func (segment *StdIn) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()
	fromStdin := make(chan []byte)
	go func() {
		for {
			scan := segment.scanner.Scan()
			if err := segment.scanner.Err(); err != nil {
				log.Warn().Err(err).Msg("StdIn: Skipping a flow, could not read line from stdin: ")
				continue
			}
			if segment.EofCloses && !scan && segment.scanner.Err() == nil {
				log.Info().Msgf("StdIn: Reached eof of %s, closing pipeline", segment.FileName)
				segment.ShutdownParentPipeline()
				return
			}
			if len(segment.scanner.Text()) == 0 {
				continue
			}
			// we need to get full representation of text and cast it to []byte
			// because scanner.Bytes doesn't return all content.
			fromStdin <- []byte(segment.scanner.Text())
		}
	}()
	for {
		select {
		case msg, ok := <-segment.In:
			if !ok {
				return
			}
			segment.Out <- msg
		case line := <-fromStdin:
			msg := &pb.EnrichedFlow{}
			err := protojson.Unmarshal(line, msg)
			if err != nil {
				log.Warn().Err(err).Msg("StdIn: Skipping a flow, failed to recode input to protobuf: ")
				continue
			}
			segment.Out <- msg
		}
	}
}

func init() {
	segment := &StdIn{}
	segments.RegisterSegment("stdin", segment)
}
