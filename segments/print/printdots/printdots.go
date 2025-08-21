// Prints a dot every n flows.
package printdots

import (
	"strconv"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/segments"
)

type PrintDots struct {
	segments.BaseTextOutputSegment
	FlowsPerDot uint64 // optional, default is 5000
}

func (segment PrintDots) New(config map[string]string) segments.Segment {
	file, err := segment.GetOutput(config)
	if err != nil {
		log.Error().Err(err).Msg("PrintDots: File specified in 'filename' is not accessible: ")
		return nil
	}
	log.Info().Msgf("PrintDots: configured output to %s", file.Name())

	var fpd uint64 = 5000
	if parsedFpd, err := strconv.ParseUint(config["flowsperdot"], 10, 32); err == nil {
		fpd = parsedFpd
	} else {
		if config["flowsperdot"] != "" {
			log.Error().Msg("PrintDots: Could not parse 'flowsperdot' parameter, using default 5000.")
		} else {
			log.Info().Msg("PrintDots: 'flowsperdot' set to default 5000.")
		}
	}
	return &PrintDots{
		FlowsPerDot: fpd,
		BaseTextOutputSegment: segments.BaseTextOutputSegment{
			File: file,
		},
	}
}

func (segment *PrintDots) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		segment.File.Close()
		wg.Done()
	}()
	count := uint64(0)
	for msg := range segment.In {
		if count += 1; count >= segment.FlowsPerDot {
			segment.File.WriteString(".")
			count = 0
		}
		segment.Out <- msg
	}
}

func init() {
	segment := &PrintDots{}
	segments.RegisterSegment("printdots", segment)
}
