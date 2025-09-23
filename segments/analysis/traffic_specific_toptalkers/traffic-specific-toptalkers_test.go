package traffic_specific_toptalkers

import (
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/pipeline"
	"github.com/BelWue/flowpipeline/pipeline/config"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/BelWue/flowpipeline/segments/testing/counter"
)

func TestSegment_TrafficSpecificToptalkers_passthrough(t *testing.T) {
	msg := &pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 123}, DstPort: 123, Packets: 1000, Bytes: 230000, Proto: 17} //Ntp (udp)
	msg2 := &pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 123}, DstPort: 443, Packets: 1, Bytes: 100, Proto: 6}

	segment := segments.LookupSegment("traffic_specific_toptalkers")
	//normally done via config
	segment.AddCustomConfig(config.SegmentRepr{
		Config: config.Config{
			ThresholdMetricDefinition: []*config.ThresholdMetricConfig{
				{
					FilterDefinition: "proto udp",
					SubDefinitions: []*config.ThresholdMetricConfig{
						{
							FilterDefinition: "port 123",
							PrometheusMetricsParamsDefinition: config.PrometheusMetricsParamsDefinition{
								TrafficType:  "NTP",
								ThresholdBps: 1,
							},
						},
					},
				},
			},
		},
	})

	segment = segment.New(map[string]string{})

	if segment == nil {
		log.Fatal().Msgf("Configured segment traffic_specific_toptalkers could not be initialized properly, see previous messages.")
	}

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	in <- msg
	resultMsg := <-out
	if resultMsg == nil {
		t.Error("Segment traffic_specific_toptalkers is not passing through flows.")
	}

	in <- msg2
	resultMsg2 := <-out
	close(in)
	wg.Wait()

	if resultMsg2 == nil {
		t.Error("Segment traffic_specific_toptalkers is not passing through flows.")
	}
}

func TestSegment_TrafficSpecificToptalkers_matching_passthrough(t *testing.T) {
	pipeline := pipeline.NewFromConfig([]byte(`---
- segment: traffic_specific_toptalkers
  config:
    endpoint: ":8085"
    traffic_specific_toptalkers:
    - filter: "proto udp"
      traffictype: "UDP"
  matching_pipeline:
  - segment: counter
`))

	var (
		counterSegment *counter.Counter
	)

	//Test if pipeline is setup correctly
	segment0 := pipeline.SegmentList[0]
	switch segment := segment0.(type) {
	case *TrafficSpecificToptalkers:
		if segment.MatchingPipeline == nil {
			t.Error("[error] subpipeline not initialized correctly")
			t.Fail()
		} else {
			subsegment0 := segment.MatchingPipeline.SegmentList[0]
			switch subsegment := subsegment0.(type) {
			case *counter.Counter:
				counterSegment = subsegment
			default:
				t.Error("[error] subpipeline not initialized correctly")
				t.Fail()
			}
		}
	default:
		t.Error("[error] pipeline not initialized correctly")
		t.Fail()
	}

	if counterSegment.Counter != 0 {
		t.Error("counter not initialized correctly")
		t.Fail()
	}
	pipeline.AutoDrain()
	pipeline.Start()

	//tcp - matched by the filter the filter
	pipeline.In <- &pb.EnrichedFlow{Proto: 17, InIf: 1, OutIf: 1, DstAddr: []byte{192, 168, 88, 142}, Packets: 15000, Bytes: 3000000}
	time.Sleep(2 * time.Second) // takes 1 db tick to get above threashold

	//tcp - not in filter
	pipeline.In <- &pb.EnrichedFlow{Proto: 6, InIf: 1, OutIf: 1, DstAddr: []byte{192, 168, 88, 143}, Packets: 15000, Bytes: 3000000}
	time.Sleep(2 * time.Second)
	if counterSegment.Counter != 0 {
		t.Error("[error] tcp packet was counted from upd matcher")
	}

	//upd in Filter
	pipeline.In <- &pb.EnrichedFlow{Proto: 17, InIf: 1, OutIf: 1, DstAddr: []byte{192, 168, 88, 142}, Packets: 15000, Bytes: 3000000}
	time.Sleep(2 * time.Second)
	if counterSegment.Counter != 1 {
		t.Error("[error] udp packet was not counted from upd matcher")
	}

	//tcp - still not in filter
	pipeline.In <- &pb.EnrichedFlow{Proto: 6, InIf: 1, OutIf: 1, DstAddr: []byte{192, 168, 88, 143}, Packets: 15000, Bytes: 3000000}
	time.Sleep(2 * time.Second)
	if counterSegment.Counter != 1 {
		t.Error("[error] tcp packet was counted from upd matcher")
	}

	//tcp, but should now be counted since the address was matched previously
	pipeline.In <- &pb.EnrichedFlow{Proto: 6, InIf: 1, OutIf: 1, DstAddr: []byte{192, 168, 88, 142}, Packets: 15000, Bytes: 3000000}
	time.Sleep(2 * time.Second)
	if counterSegment.Counter != 2 {
		t.Error("[error] tcp packet was not counted from upd matcher")
	}
	pipeline.Close()
}
