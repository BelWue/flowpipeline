package traffic_specific_toptalkers

import (
	"sync"
	"testing"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/pipeline/config"
	"github.com/BelWue/flowpipeline/segments"
)

func TestSegment_Branch_passthrough(t *testing.T) {
	msg := &pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 123}, DstPort: 123, Packets: 1000, Bytes: 230000, Proto: 17} //Ntp (udp)
	msg2 := &pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 123}, DstPort: 443, Packets: 1, Bytes: 100, Proto: 6}

	segment := segments.LookupSegment("traffic_specific_toptalkers")
	//normally done via config
	segment.AddCustomConfig(config.Config{
		ThresholdMetricDefinition: []*config.ThresholdMetricDefinition{
			{
				FilterDefinition: "proto udp",
				SubDefinitions: []*config.ThresholdMetricDefinition{
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
