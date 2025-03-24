package branch

import (
	"sync"
	"testing"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
)

// Branch Segment test, passthrough test
// This does not work currently, as segment tests are scoped for segment
// package only, and this specific segment requires some pipeline
// initialization, which would lead to an import cycle. Thus, this test
// confirms that it fails silently, and this segment is instead tested from the
// pipeline package test files.
func TestSegment_Branch_passthrough(t *testing.T) {
	segment := segments.LookupSegment("branch").New(map[string]string{}).(*Branch)
	if segment == nil {
		log.Fatal().Msg("Configured segment 'branch' could not be initialized properly, see previous messages.")
	}
	segment.condition = NewMockPipeline()
	segment.then_branch = NewMockPipeline()
	segment.else_branch = NewMockPipeline()

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)
	in <- &pb.EnrichedFlow{}
	result := <-segment.else_branch.GetDrop() //condition drops --> else branch --> drops
	if result == nil {
		t.Error("Segment Goflow is not dropping flows as expected.")
	}
}

// If bypass is enabled, that branch forwards every incoming message regardless of the internal segments
func TestSegment_Branch_passthrough_bypass(t *testing.T) {
	segment := segments.LookupSegment("branch").New(map[string]string{"bypass-messages": "true"}).(*Branch)
	if segment == nil {
		log.Fatal().Msg("Configured segment 'branch' could not be initialized properly, see previous messages.")
	}
	segment.condition = NewMockPipeline()
	segment.then_branch = NewMockPipeline()
	segment.else_branch = NewMockPipeline()

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)
	in <- &pb.EnrichedFlow{}
	result := <-out
	if result == nil {
		t.Error("Segment Goflow is not passing through flows.")
	}
	close(in)
}

func NewMockPipeline() Pipeline {
	mock := &MockPipeline{}
	mock.Drop = make(chan *pb.EnrichedFlow)
	mock.In = make(chan *pb.EnrichedFlow)
	mock.Out = make(chan *pb.EnrichedFlow)
	return mock
}

// Mock implementation dropping every message
type MockPipeline struct {
	In   chan *pb.EnrichedFlow
	Out  <-chan *pb.EnrichedFlow
	Drop chan *pb.EnrichedFlow
}

func (m *MockPipeline) Start() {
	for msg := range m.In { // connect our own input to conditional
		m.Drop <- msg
	}
}
func (m *MockPipeline) Close() {
	defer func() {
		recover() // in case In is already closed
	}()
	close(m.In)
}
func (m *MockPipeline) GetInput() chan *pb.EnrichedFlow {
	return m.In
}
func (m *MockPipeline) GetOutput() <-chan *pb.EnrichedFlow {
	return m.Out
}
func (m *MockPipeline) GetDrop() <-chan *pb.EnrichedFlow {
	return m.Drop
}
