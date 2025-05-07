package anonymize

import (
	"net"
	"os"
	"sync"
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/rs/zerolog"
)

// Influx Segment test, passthrough test only
func TestSegment_Influx_passthrough(t *testing.T) {
	result := segments.TestSegment("anonymize", map[string]string{"key": "testkey123jfh789fhj456ezhskila73"},
		&pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 123}, NextHop: []byte{193, 168, 88, 2}})
	if result == nil {
		t.Error("([error] Segment Anonymize is not passing through flows.")
	}
}

// Anonymize Segment benchmark passthrough
func BenchmarkAnonymize(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Stdout, _ = os.Open(os.DevNull)
	segment := Anonymize{}.New(map[string]string{
		"key":    "ExampleKeyWithExactly32Character",
		"fields": "SrcAddr,DstAddr,NextHop",
		"mode":   "all",
	})
	if segment == nil {
		b.Error("Failed to init Anonymize segment")
	}
	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 123}, NextHop: []byte{193, 168, 88, 2}}
		<-out
	}
	close(in)
}

func TestCryptoPan(t *testing.T) {
	os.Stdout, _ = os.Open(os.DevNull)
	segment := Anonymize{}.New(map[string]string{
		"key":    "ExampleKeyWithExactly32Character",
		"fields": "SrcAddr,DstAddr,NextHop",
		"mode":   "cryptopan",
	})
	if segment == nil {
		t.Error("Failed to init Anonymize segment")
	}

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	in <- &pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 123}, NextHop: []byte{193, 168, 88, 2}}
	msg := <-out
	if net.IP(msg.SrcAddr).String() != "71.207.64.145" {
		t.Errorf("Wrong Crypto-PAn SrcAddr %s  - 71.207.64.145 expected", net.IP(msg.SrcAddr).String())
	}
	if net.IP(msg.DstAddr).String() != "71.207.64.4" {
		t.Errorf("Wrong Crypto-PAn DstAddr %s  - 71.207.64.4 expected", net.IP(msg.DstAddr).String())
	}
	if net.IP(msg.NextHop).String() != "70.87.71.154" {
		t.Errorf("Wrong Crypto-PAn NextHop %s  - 70.87.71.154 expected", net.IP(msg.NextHop).String())
	}
	if msg.SrcAddrAnon != pb.EnrichedFlow_CryptoPAN {
		t.Error("Wrong Meta Field msg.SrcAddrAnon")
	}
	if msg.DstAddrAnon != pb.EnrichedFlow_CryptoPAN {
		t.Error("Wrong Meta Field msg.DstAddrAnon")
	}
	if msg.NextHopAnon != pb.EnrichedFlow_CryptoPAN {
		t.Error("Wrong Meta Field msg.NextHopAnon")
	}
	if msg.SamplerAddrAnon != pb.EnrichedFlow_NotAnonymized {
		t.Error("Wrong Meta Field msg.SamplerAddrAnon")
	}
	close(in)
}

func TestSubnet(t *testing.T) {
	os.Stdout, _ = os.Open(os.DevNull)
	segment := Anonymize{}.New(map[string]string{
		"key":    "ExampleKeyWithExactly32Character",
		"fields": "SrcAddr,DstAddr,NextHop",
		"mode":   "subnet",
		"maskV4": "24",
		"maskV6": "112",
	})
	if segment == nil {
		t.Error("Failed to init Anonymize segment")
	}

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	in <- &pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 123}, NextHop: []byte{193, 168, 88, 2}}
	msg := <-out

	if net.IP(msg.SrcAddr).String() != "192.168.88.0" {
		t.Errorf("Wrong subnet addresses %s  - 192.168.88.0 expected", net.IP(msg.SrcAddr).String())
	}
	if net.IP(msg.DstAddr).String() != "192.168.88.0" {
		t.Errorf("Wrong subnet addresses %s  - 192.168.88.0 expected", net.IP(msg.DstAddr).String())
	}
	if net.IP(msg.NextHop).String() != "193.168.88.0" {
		t.Errorf("Wrong subnet addresses %s  - 193.168.88.0 expected", net.IP(msg.NextHop).String())
	}

	if msg.DstAddrPreservedLen != 24 {
		t.Errorf("Wrong DstAddrPreservedLen %d  - 24 expected", msg.DstAddrPreservedLen)
	}
	if msg.SrcAddrPreservedLen != 24 {
		t.Errorf("Wrong SrcAddrPreservedLen %d  - 24 expected", msg.SrcAddrPreservedLen)
	}
	if msg.NextHopAnonPreservedPrefixLen != 24 {
		t.Errorf("Wrong DstAddrPreservedLen %d  - 24 expected", msg.NextHopAnonPreservedPrefixLen)
	}

	if msg.SrcAddrAnon != pb.EnrichedFlow_Subnet {
		t.Errorf("Wrong Meta Field msg.SrcAddrAnon %s ", msg.SrcAddrAnon)
	}
	if msg.DstAddrAnon != pb.EnrichedFlow_Subnet {
		t.Errorf("Wrong Meta Field msg.DstAddrAnon %s", msg.DstAddrAnon)
	}
	if msg.NextHopAnon != pb.EnrichedFlow_Subnet {
		t.Errorf("Wrong Meta Field msg.NextHopAnon %s", msg.NextHopAnon)
	}
	if msg.SamplerAddrAnon != pb.EnrichedFlow_NotAnonymized {
		t.Errorf("Wrong Meta Field msg.SamplerAddrAnon %s", msg.SamplerAddrAnon)
	}
	close(in)
}

func TestBoth(t *testing.T) {
	os.Stdout, _ = os.Open(os.DevNull)
	segment := Anonymize{}.New(map[string]string{
		"key":    "ExampleKeyWithExactly32Character",
		"fields": "SrcAddr,DstAddr,NextHop",
		"mode":   "all",
		"maskV4": "24",
		"maskV6": "112",
	})
	if segment == nil {
		t.Error("Failed to init Anonymize segment")
	}

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	in <- &pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 123}, NextHop: []byte{193, 168, 88, 2}}
	msg := <-out
	if net.IP(msg.SrcAddr).String() != "71.207.64.0" {
		t.Errorf("Wrong Crypto-PAn SrcAddr %s  - 71.207.64.0 expected", net.IP(msg.SrcAddr).String())
	}
	if net.IP(msg.DstAddr).String() != "71.207.64.0" {
		t.Errorf("Wrong Crypto-PAn DstAddr %s  - 71.207.64.0 expected", net.IP(msg.DstAddr).String())
	}
	if net.IP(msg.NextHop).String() != "70.87.71.0" {
		t.Errorf("Wrong Crypto-PAn NextHop %s  - 70.87.71.0 expected", net.IP(msg.NextHop).String())
	}

	if msg.DstAddrPreservedLen != 24 {
		t.Errorf("Wrong DstAddrPreservedLen %d  - 24 expected", msg.DstAddrPreservedLen)
	}
	if msg.SrcAddrPreservedLen != 24 {
		t.Errorf("Wrong SrcAddrPreservedLen %d  - 24 expected", msg.SrcAddrPreservedLen)
	}
	if msg.NextHopAnonPreservedPrefixLen != 24 {
		t.Errorf("Wrong DstAddrPreservedLen %d  - 24 expected", msg.NextHopAnonPreservedPrefixLen)
	}

	if msg.SrcAddrAnon != pb.EnrichedFlow_SubnetAndCryptoPAN {
		t.Errorf("Wrong Meta Field msg.SrcAddrAnon %s ", msg.SrcAddrAnon)
	}
	if msg.DstAddrAnon != pb.EnrichedFlow_SubnetAndCryptoPAN {
		t.Errorf("Wrong Meta Field msg.DstAddrAnon %s", msg.DstAddrAnon)
	}
	if msg.NextHopAnon != pb.EnrichedFlow_SubnetAndCryptoPAN {
		t.Errorf("Wrong Meta Field msg.NextHopAnon %s", msg.NextHopAnon)
	}
	if msg.SamplerAddrAnon != pb.EnrichedFlow_NotAnonymized {
		t.Errorf("Wrong Meta Field msg.SamplerAddrAnon %s", msg.SamplerAddrAnon)
	}
	close(in)
}

func BenchmarkSubnet_1000(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := Anonymize{}.New(map[string]string{
		"fields": "SrcAddr,DstAddr,NextHop",
		"mode":   "subnet",
		"maskV4": "24",
		"maskV6": "112",
	})
	if segment == nil {
		b.Error("Failed to init Anonymize segment")
	}

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 143}, NextHop: []byte{192, 168, 88, 143}, Proto: 45}
		<-out
	}
	close(in)
}

func BenchmarkCryptopan_1000(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := Anonymize{}.New(map[string]string{
		"key":    "ExampleKeyWithExactly32Character",
		"fields": "SrcAddr,DstAddr,NextHop",
		"mode":   "cryptopan",
	})

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 143}, NextHop: []byte{192, 168, 88, 143}, Proto: 45}
		<-out
	}
	close(in)
}
