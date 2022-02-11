package sqlite

import (
	"io/ioutil"
	"log"
	"os"
	"sync"
	"testing"

	// "github.com/bwNetFlow/flowpipeline/segments"
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

// Sqlite Segment test, passthrough test only
func TestSegment_Sqlite_passthrough(t *testing.T) {
	// result := segments.TestSegment("sqlite", map[string]string{"filename": "test.sqlite"},
	// 	&flow.FlowMessage{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 143}, Proto: 45})
	// if result == nil {
	// 	t.Error("Segment Sqlite is not passing through flows.")
	// }
	segment := Sqlite{}.New(map[string]string{"filename": "test.sqlite"})

	in, out := make(chan *flow.FlowMessage), make(chan *flow.FlowMessage)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)
	in <- &flow.FlowMessage{SrcAddr: []byte{192, 168, 88, 1}, DstAddr: []byte{192, 168, 88, 1}, Proto: 1}
	<-out
	in <- &flow.FlowMessage{SrcAddr: []byte{192, 168, 88, 2}, DstAddr: []byte{192, 168, 88, 2}, Proto: 2}
	<-out
	close(in)
	wg.Wait()
}

// Sqlite Segment benchmark with 1000 samples stored in memory
func BenchmarkSqlite_1000(b *testing.B) {
	log.SetOutput(ioutil.Discard)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := Sqlite{}.New(map[string]string{"filename": "bench.sqlite"})

	in, out := make(chan *flow.FlowMessage), make(chan *flow.FlowMessage)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &flow.FlowMessage{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 143}, Proto: 45}
		_ = <-out
	}
	close(in)
}

// Sqlite Segment benchmark with 10000 samples stored in memory
func BenchmarkSqlite_10000(b *testing.B) {
	log.SetOutput(ioutil.Discard)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := Sqlite{}.New(map[string]string{"filename": "bench.sqlite", "batchsize": "10000"})

	in, out := make(chan *flow.FlowMessage), make(chan *flow.FlowMessage)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &flow.FlowMessage{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 143}, Proto: 45}
		_ = <-out
	}
	close(in)
}

// Sqlite Segment benchmark with 10000 samples stored in memory
func BenchmarkSqlite_100000(b *testing.B) {
	log.SetOutput(ioutil.Discard)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := Sqlite{}.New(map[string]string{"filename": "bench.sqlite", "batchsize": "100000"})

	in, out := make(chan *flow.FlowMessage), make(chan *flow.FlowMessage)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &flow.FlowMessage{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 143}, Proto: 45}
		_ = <-out
	}
	close(in)
}
