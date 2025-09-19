// TODO: Compile this using:
// `go build -buildmode=plugin ./examples/configuration/plugin/printcustom.go`
package main

import (
	"fmt"
	"sync"

	"github.com/BelWue/flowpipeline/segments"
)

// This is a short example on how to write custom segments and load them as a plugin.
// Please do not edit anything except the specified lines if you are unsure of
// what you're doing.

// TODO: This type name can be edited to your liking and has to be replaced
// thourough the segment.
type PrintCustom struct {
	segments.BaseSegment
}

func (segment PrintCustom) New(config map[string]string) segments.Segment {
	// This space is for parsing configuration and failing early if
	// something is wrong. It is left empty for now.
	return &PrintCustom{}
}

func (segment *PrintCustom) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()
	// TODO: Edit to your liking from here on up until the end of this
	// method.
	for msg := range segment.In {
		switch b := msg.Bytes * 8; {
		case b < 102400:
			fmt.Printf("cute little flow with %d bits!\n", b)
		case b < 204800:
			fmt.Printf("nice flow with %d bits!\n", b)
		case b < 409600:
			fmt.Printf("solid flow with %d bits!\n", b)
		case b < 819200:
			fmt.Printf("impressive flow with %d bits!\n", b)
		case b >= 819200:
			fmt.Printf("hefty flow with %d bits!\n", b)
		}
		// TODO: not doing this will drop flows, try moving it into
		// some case above.
		segment.Out <- msg
	}
}

func init() {
	segment := &PrintCustom{}
	// TODO: edit the name you'll use in your config file here.
	segments.RegisterSegment("printcustom", segment)
}
