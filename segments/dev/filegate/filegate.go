// Serves as a template for new segments and forwards flows, otherwise does
// nothing.
package filegate

import (
	"errors"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/segments"
)

type Filegate struct {
	segments.BaseSegment // always embed this, no need to repeat I/O chan code
	filename             string
}

// Every Segment must implement a New method, even if there isn't any config
// it is interested in.
func (segment *Filegate) New(config map[string]string) segments.Segment {
	var ()
	if config["filename"] != "" {
		segment.filename = config["filename"]
		log.Info().Msgf("Filegate: gate file is %s", segment.filename)
	} else {
		log.Fatal().Msgf("Filegate: No filename config option")
	}
	// do config stuff here, add it to fields maybe
	return segment
}

func checkFileExists(filename string) bool {
	log.Debug().Msgf("Filegate: check if filename %s exists", filename)
	_, err := os.Stat(filename)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

func (segment *Filegate) Run(wg *sync.WaitGroup) {
	defer func() {
		// This defer clause is important and needs to be present in
		// any Segment.Run method in some form, but with at least the
		// following two statements.
		close(segment.Out)
		wg.Done()
	}()
	for msg := range segment.In {
		for checkFileExists(segment.filename) {
			log.Info().Msgf("Filegate: gate file %s exists", segment.filename)
			time.Sleep(2 * time.Second)
		}
		segment.Out <- msg
	}
}

func init() {
	segment := &Filegate{}
	segments.RegisterSegment("filegate", segment)
}
