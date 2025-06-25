package pipeline

import (
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/BelWue/flowpipeline/segments/pass"

	_ "github.com/BelWue/flowpipeline/segments/filter/drop"
	_ "github.com/BelWue/flowpipeline/segments/filter/flowfilter"
	_ "github.com/BelWue/flowpipeline/segments/modify/dropfields"
	_ "github.com/BelWue/flowpipeline/segments/testing/generator"
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

func Test_Branch_passthrough(t *testing.T) {
	pipeline := NewFromConfig([]byte(`---
- segment: branch
  if:
  - segment: flowfilter
    config:
      filter: proto tcp
  then:
  - segment: dropfields
    config:
      policy: drop
      fields: InIf
  else:
  - segment: dropfields
    config:
      policy: drop
      fields: OutIf
`))
	pipeline.Start()
	pipeline.In <- &pb.EnrichedFlow{Proto: 6, InIf: 1, OutIf: 1}
	fmsg := <-pipeline.Out
	if fmsg.Proto != 6 || fmsg.InIf == 1 || fmsg.OutIf != 1 {
		t.Errorf("[error] Branch segment did not work correctly, state is Proto %d, InIf %d, OutIf %d, should be (6, 0, 1).", fmsg.Proto, fmsg.InIf, fmsg.OutIf)
	}
	pipeline.In <- &pb.EnrichedFlow{Proto: 42, InIf: 1, OutIf: 1}
	fmsg = <-pipeline.Out
	if fmsg.Proto != 42 || fmsg.InIf != 1 || fmsg.OutIf == 1 {
		t.Errorf("[error] Branch segment did not work correctly, state is Proto %d, InIf %d, OutIf %d, should be (42, 1, 0).", fmsg.Proto, fmsg.InIf, fmsg.OutIf)
	}
}

func Test_Branch_DeadlockFreeGeneration_If(t *testing.T) {
	pipeline := NewFromConfig([]byte(`---
- segment: branch
  if:
  - segment: generator
  - segment: flowfilter
    config:
      filter: proto tcp
  then:
  - segment: dropfields
    config:
      policy: drop
      fields: Bytes
`))
	pipeline.Start()
	pipeline.In <- &pb.EnrichedFlow{Proto: 42, Bytes: 42}
	for i := 0; i < 5; i++ {
		fmsg := <-pipeline.Out
		if fmsg.Proto == 6 && fmsg.Bytes != 0 {
			t.Errorf("[error] Branch segment did not work correctly, state is Proto %d, Bytes %d, should be (6, 0).", fmsg.Proto, fmsg.Bytes)
		} else if fmsg.Proto == 42 && fmsg.Bytes != 42 {
			t.Errorf("[error] Branch segment did not work correctly, state is Proto %d, Bytes %d, should be (42, 42).", fmsg.Proto, fmsg.Bytes)
		}
	}
}

func Test_Branch_DeadlockFreeGeneration_Then(t *testing.T) {
	pipeline := NewFromConfig([]byte(`---
- segment: branch
  then:
  - segment: generator
`))
	pipeline.Start()
	pipeline.In <- &pb.EnrichedFlow{Proto: 42, Bytes: 42}
	for i := 0; i < 5; i++ {
		// no checks, not timeouting is enough
		<-pipeline.Out
	}
}

func Test_Branch_DeadlockFreeGeneration_Else(t *testing.T) {
	pipeline := NewFromConfig([]byte(`---
- segment: branch
  else:
  - segment: generator
`))
	pipeline.Start()
	pipeline.In <- &pb.EnrichedFlow{Proto: 42, Bytes: 42}
	for i := 0; i < 5; i++ {
		// no checks, not timeouting is enough
		<-pipeline.Out
	}
}
