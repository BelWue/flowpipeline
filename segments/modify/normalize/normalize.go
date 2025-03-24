// Rewrites passing flows with all sampling rate affected fields normalized.
package normalize

import (
	"strconv"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/segments"
)

type Normalize struct {
	segments.BaseSegment
	Fallback uint64 // optional, default is no fallback, determines a assumed sampling rate of flows if none is found in a given flow
}

func (segment Normalize) New(config map[string]string) segments.Segment {
	var fallback uint64 = 0

	if config["fallback"] != "" {
		if parsedFallback, err := strconv.ParseUint(config["fallback"], 10, 32); err == nil {
			fallback = parsedFallback
		} else {
			log.Error().Msg("Normalize: Could not parse 'fallback' parameter, using default 0.")
		}
	} else {
		log.Info().Msg("Normalize: 'fallback' set to default '0'.")
	}

	return &Normalize{
		Fallback: fallback,
	}
}

func (segment *Normalize) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()
	for msg := range segment.In {
		if msg.SamplingRate > 0 {
			msg.Bytes *= msg.SamplingRate
			msg.Packets *= msg.SamplingRate
			msg.Normalized = 1
		} else if segment.Fallback != 0 {
			msg.Bytes *= segment.Fallback
			msg.Packets *= segment.Fallback
			msg.SamplingRate = segment.Fallback
			msg.Normalized = 1
		}
		segment.Out <- msg
	}
}

func init() {
	segment := &Normalize{}
	segments.RegisterSegment("normalize", segment)
}
