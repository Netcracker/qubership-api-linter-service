package service

import (
	"reflect"
	"testing"
)

func TestDiffMembers(t *testing.T) {
	const (
		selfID    uint64 = 1
		peerID    uint64 = 2
		newPeerID uint64 = 3
	)

	tests := []struct {
		name       string
		previous   map[uint64]string
		current    map[uint64]string
		wantJoined map[uint64]string
		wantLeft   map[uint64]string
	}{
		{
			name:       "no change",
			previous:   map[uint64]string{selfID: "self", peerID: "peer"},
			current:    map[uint64]string{selfID: "self", peerID: "peer"},
			wantJoined: map[uint64]string{},
			wantLeft:   map[uint64]string{},
		},
		{
			name:       "peer left",
			previous:   map[uint64]string{selfID: "self", peerID: "peer"},
			current:    map[uint64]string{selfID: "self"},
			wantJoined: map[uint64]string{},
			wantLeft:   map[uint64]string{peerID: "peer"},
		},
		{
			name:       "peer joined",
			previous:   map[uint64]string{selfID: "self"},
			current:    map[uint64]string{selfID: "self", peerID: "peer"},
			wantJoined: map[uint64]string{peerID: "peer"},
			wantLeft:   map[uint64]string{},
		},
		{
			name:       "peer restarted between polls gets a new ID",
			previous:   map[uint64]string{selfID: "self", peerID: "peer"},
			current:    map[uint64]string{selfID: "self", newPeerID: "peer"},
			wantJoined: map[uint64]string{newPeerID: "peer"},
			wantLeft:   map[uint64]string{peerID: "peer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			joined, left := diffMembers(tt.previous, tt.current)
			if !reflect.DeepEqual(joined, tt.wantJoined) {
				t.Errorf("joined = %v, want %v", joined, tt.wantJoined)
			}
			if !reflect.DeepEqual(left, tt.wantLeft) {
				t.Errorf("left = %v, want %v", left, tt.wantLeft)
			}
		})
	}
}
