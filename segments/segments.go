// This package is home to all pipeline segment implementations. Generally,
// every segment lives in its own package, implements the Segment interface,
// embeds the BaseSegment to take care of the I/O side of things, and has an
// additional init() function to register itself using RegisterSegment.
package segments

import (
	"sync"
	"syscall"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/pipeline/config"
	"github.com/rs/zerolog/log"
)

var (
	registeredSegments    = make(map[string]Segment)
	lock                  = &sync.RWMutex{}
	ContainerVolumePrefix = ""
)

// Used by Segments to register themselves in their init() functions. Errors
// and exits immediately on conflicts.
func RegisterSegment(name string, s Segment) {
	_, ok := registeredSegments[name]
	if ok {
		log.Fatal().Msgf("Segments: Tried to register conflicting segment name '%s'.", name)
	}
	lock.Lock()
	registeredSegments[name] = s
	lock.Unlock()
}

// Used by the pipeline package to convert segment names in configuration to
// actual Segment objects.
func LookupSegment(name string) Segment {
	lock.RLock()
	segment, ok := registeredSegments[name]
	lock.RUnlock()
	if !ok {
		log.Fatal().Msgf("Segments: Could not find a segment named '%s'.", name)
	}
	return segment
}

// Used by the tests to run single flow messages through a segment.
func TestSegment(name string, config map[string]string, msg *pb.EnrichedFlow) *pb.EnrichedFlow {
	segment := LookupSegment(name).New(config)
	if segment == nil {
		log.Fatal().Msgf("Configured segment '%s' could not be initialized properly, see previous messages.", name)
	}

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	in <- msg
	close(in)
	resultMsg := <-out
	wg.Wait()

	return resultMsg
}

// This interface is central to an Pipeline object, as it operates on a list of
// them. In general, Segments should embed the BaseSegment to provide the
// Rewire function and the associated vars.
type Segment interface {
	New(config map[string]string) Segment                       // for reading the provided config
	Run(wg *sync.WaitGroup)                                     // goroutine, must close(segment.Out) when segment.In is closed
	Rewire(in chan *pb.EnrichedFlow, out chan *pb.EnrichedFlow) // embed this using BaseSegment
	ShutdownParentPipeline()                                    // shut down Parent Pipeline gracefully
	AddCustomConfig(config config.Config)                       //Add segment specific sturctured config parameters
	Close()
}

// Serves as a basis for any Segment implementations. Segments embedding this
// type only need the New and the Run methods to be compliant to the Segment
// interface.
type BaseSegment struct {
	In  <-chan *pb.EnrichedFlow
	Out chan<- *pb.EnrichedFlow
}

// This function rewires this Segment with the provided channels. This is
// typically called only by pipeline.New() and present in any Segment
// implementation embedding the BaseSegment.
// The peculiar implementation of passing the full channel list and providing
// indexes is due to the fact that controlflow segments may want to skip
// segments and thus need to have all later references available as well.
func (segment *BaseSegment) Rewire(in chan *pb.EnrichedFlow, out chan *pb.EnrichedFlow) {
	segment.In = in
	segment.Out = out
}

// This functions shutdown Parent Pipeline segments on the given syscall.
// It is used for intended termination within pipeline function, e.g. end pipeline on read from file.
func (segment *BaseSegment) ShutdownParentPipeline() {
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)
}

func (segment *BaseSegment) Close() {
	//placeholder since most segments dont need to do anything
}

func (segment *BaseSegment) AddCustomConfig(config.Config) {
	//placeholder since most segments dont have a custom sturctured config
}
