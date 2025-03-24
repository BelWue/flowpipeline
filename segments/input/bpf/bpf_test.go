//go:build linux && cgo
// +build linux,cgo

package bpf

// Bpf Segment test, passthrough test TODO: how to guarantee device presence on any host
// func TestSegment_Bpf_passthrough(t *testing.T) {
// 	result := segments.TestSegment("bpf", map[string]string{"device": "eth0"},
// 		&pb.EnrichedFlow{Type: 3})
// 	if result.Type != 3 {
// 		t.Error("([error] Segment Bpf is not working.")
// 	}
// }
