//go:build cgo
// +build cgo

package sqlite

import (
	"os"
	"sync"
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/rs/zerolog"
	// "github.com/BelWue/flowpipeline/segments"
)

// Sqlite Segment test, passthrough test only
func TestSegment_Sqlite_passthrough(t *testing.T) {
	// result := segments.TestSegment("sqlite", map[string]string{"filename": "test.sqlite"},
	// 	&pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 143}, Proto: 45})
	// if result == nil {
	// 	t.Error("([error] Segment Sqlite is not passing through flows.")
	// }
	segment := Sqlite{}.New(map[string]string{"filename": "test.sqlite"})

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)
	in <- &pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 1}, DstAddr: []byte{192, 168, 88, 1}, Proto: 1}
	<-out
	in <- &pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 2}, DstAddr: []byte{192, 168, 88, 2}, Proto: 2}
	<-out
	close(in)
	wg.Wait()
}

// Sqlite Segment benchmark with 1000 samples stored in memory
func BenchmarkSqlite_1000(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := Sqlite{}.New(map[string]string{"filename": "bench.sqlite"})

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 143}, Proto: 45}
		<-out
	}
	close(in)
}

// Sqlite Segment benchmark with 10000 samples stored in memory
func BenchmarkSqlite_10000(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := Sqlite{}.New(map[string]string{"filename": "bench.sqlite", "batchsize": "10000"})

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 143}, Proto: 45}
		<-out
	}
	close(in)
}

// Sqlite Segment benchmark with 10000 samples stored in memory
func BenchmarkSqlite_100000(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := Sqlite{}.New(map[string]string{"filename": "bench.sqlite", "batchsize": "100000"})

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 143}, Proto: 45}
		<-out
	}
	close(in)
}
