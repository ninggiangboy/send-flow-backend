package id

import (
	"bytes"
	"testing"
)

func TestUUIDGenerator_NewReturnsVersion4UUID(t *testing.T) {
	reader := bytes.NewReader([]byte{
		0x00, 0x01, 0x02, 0x03,
		0x04, 0x05,
		0x06, 0x07,
		0x08, 0x09,
		0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	})
	got, err := NewUUIDGeneratorWithReader(reader).New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "00010203-0405-4607-8809-0a0b0c0d0e0f" {
		t.Fatalf("unexpected uuid: %s", got)
	}
	if !IsUUID(got) {
		t.Fatalf("expected valid uuid")
	}
}
