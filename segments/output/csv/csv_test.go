package csv

import (
	"io/ioutil"
	"log"
	"net"
	"os"
	"sync"
	"testing"

	"github.com/bwNetFlow/flowpipeline/segments"
	flow "github.com/bwNetFlow/protobuf/go"
	"github.com/hashicorp/logutils"
)

func TestMain(m *testing.M) {
	log.SetOutput(&logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"info", "warning", "error"},
		MinLevel: logutils.LogLevel("info"),
		Writer:   os.Stderr,
	})
	code := m.Run()
	os.Exit(code)
}

// Csv Segment test, passthrough test
func TestSegment_Csv_passthrough(t *testing.T) {
	result := segments.TestSegment("csv", map[string]string{},
		&flow.FlowMessage{Type: 3, SamplerAddress: net.ParseIP("192.0.2.1")})

	if result.Type != 3 {
		t.Error("Segment Csv is not working.")
	}
}

// Csv Segment benchmark passthrough
func BenchmarkCsv(b *testing.B) {
	log.SetOutput(ioutil.Discard)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := Csv{}.New(map[string]string{})

	in, out := make(chan *flow.FlowMessage), make(chan *flow.FlowMessage)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &flow.FlowMessage{Proto: 45}
		_ = <-out
	}
	close(in)
}
