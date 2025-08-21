// This package is home to all pipeline segment implementations. Generally,
// every segment lives in its own package, implements the Segment interface,
// embeds the BaseSegment to take care of the I/O side of things, and has an
// additional init() function to register itself using RegisterSegment.
package segments

import (
	"os"
)

type TextOutputSegment interface {
	Segment
	GetOutput(config map[string]string) (*os.File, error)
}

// An extended basis for Segment implementations in the filter group. It
// contains the necessities to process filtered (dropped) flows.
type BaseTextOutputSegment struct {
	BaseSegment
	File *os.File // optional, default is empty which means stdout
}

func (s *BaseTextOutputSegment) GetOutput(config map[string]string) (*os.File, error) {
	var err error
	if config["filename"] != "" {
		s.File, err = os.Create(config["filename"])
		if err != nil {
			return nil, err
		}
	} else {
		s.File = os.Stdout
	}
	return s.File, nil
}
