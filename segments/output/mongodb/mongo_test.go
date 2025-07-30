//go:build cgo
// +build cgo

package mongodb

import (
	"os"
	"sync"
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/rs/zerolog"
	// "github.com/BelWue/flowpipeline/segments"
)

// Mongodb Segment test, passthrough test only
func TestSegment_Mongodb_passthrough(t *testing.T) {
	// result := segments.TestSegment("Mongodb", map[string]string{"mongodb_uri": "mongodb://localhost:27017/" , "database":"testing"},
	// 	&pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 143}, Proto: 45})
	// if result == nil {
	// 	t.Error("([error] Segment Mongodb is not passing through flows.")
	// }
	segment := Mongodb{}.New(map[string]string{"mongodb_uri": "mongodb://localhost:27017/", "database": "testing"})
	if segment == nil {
		t.Skip()
	}

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

// Mongodb Segment benchmark with 1000 samples stored in memory
func BenchmarkMongodb_1000(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := Mongodb{}.New(map[string]string{"mongodb_uri": "mongodb://localhost:27017/", "database": "testing"})
	if segment == nil {
		b.Skip()
	}

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

// Mongodb Segment benchmark with 10000 samples stored in memory
func BenchmarkMongodb_10000(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := Mongodb{}.New(map[string]string{"mongodb_uri": "mongodb://localhost:27017/", "database": "testing", "batchsize": "10000"})
	if segment == nil {
		b.Skip()
	}

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

// Mongodb Segment benchmark with 10000 samples stored in memory
func BenchmarkMongodb_100000(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := Mongodb{}.New(map[string]string{"mongodb_uri": "mongodb://localhost:27017/", "database": "testing", "batchsize": "1000"})
	if segment == nil {
		b.Skip()
	}

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

// Mongodb Segment benchmark with 10000 samples stored in memory
func BenchmarkMongodb_100000_with_storage_limit(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := Mongodb{}.New(map[string]string{
		"mongodb_uri":    "mongodb://localhost:27017/",
		"database":       "testing",
		"batchsize":      "100",
		"max_disk_usage": "100 MB"})
	if segment == nil {
		b.Skip()
	}

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
