package pipeline

import (
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/BelWue/flowpipeline/segments/pass"
)

func TestPipelineBuild(t *testing.T) {
	segmentList := []segments.Segment{&pass.Pass{}, &segments.ParallelizedSegment{}}

	parallelizedSegment, _ := (segmentList[1]).(*segments.ParallelizedSegment)
	parallelizedSegment.AddSegment(&pass.Pass{})

	pipeline := New(segmentList...)
	pipeline.Start()
	pipeline.In <- &pb.EnrichedFlow{Type: 3}
	fmsg := <-pipeline.Out
	if fmsg.Type != 3 {
		t.Error("([error] Pipeline Setup is not working.")
	}
}

func TestPipelineTeardown(t *testing.T) {
	segmentList := []segments.Segment{&pass.Pass{}, &segments.ParallelizedSegment{}}

	parallelizedSegment, _ := (segmentList[1]).(*segments.ParallelizedSegment)
	parallelizedSegment.AddSegment(&pass.Pass{})

	pipeline := New(segmentList...)
	pipeline.Start()
	pipeline.AutoDrain()
	pipeline.In <- &pb.EnrichedFlow{Type: 3}
	pipeline.Close() // fail test on halting ;)
}

func TestPipelineConfigSuccess(t *testing.T) {
	pipeline := NewFromConfig([]byte(`---
- segment: pass
  config:
    foo: $baz
    bar: $0`))
	pipeline.Start()
	pipeline.In <- &pb.EnrichedFlow{Type: 3}
	fmsg := <-pipeline.Out
	if fmsg.Type != 3 {
		t.Error("([error] Pipeline built from config is not working.")
	}
}
