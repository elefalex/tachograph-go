package dd

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	ddv1 "github.com/way-platform/tachograph-go/proto/gen/go/wayplatform/connect/tachograph/dd/v1"
)

func TestUnmarshalBcdString(t *testing.T) {
	tests := []struct {
		name       string
		input      []byte
		wantValue  int32
		wantLength int32
		wantErr    bool
	}{
		{
			name:       "two-byte BCD",
			input:      []byte{0x12, 0x34},
			wantValue:  1234,
			wantLength: 2,
		},
		{
			name:       "three-byte BCD",
			input:      []byte{0x12, 0x34, 0x56},
			wantValue:  123456,
			wantLength: 3,
		},
		{
			name:       "single byte",
			input:      []byte{0x99},
			wantValue:  99,
			wantLength: 1,
		},
		{
			name:       "zero value",
			input:      []byte{0x00},
			wantValue:  0,
			wantLength: 1,
		},
		{
			name:       "leading zeros",
			input:      []byte{0x00, 0x42},
			wantValue:  42,
			wantLength: 2,
		},
		{
			name:    "empty input",
			input:   []byte{},
			wantErr: true,
		},
		{
			name:    "invalid BCD characters",
			input:   []byte{0xab, 0xcd},
			wantErr: true,
		},
		{
			name:    "partially invalid BCD",
			input:   []byte{0x12, 0xfa},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unmarshalOpts := UnmarshalOptions{PreserveRawData: true}
			got, err := unmarshalOpts.UnmarshalBcdString(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("UnmarshalBcdString() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("UnmarshalBcdString() unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("UnmarshalBcdString() returned nil")
			}
			if got.GetValue() != tt.wantValue {
				t.Errorf("UnmarshalBcdString().GetValue() = %v, want %v", got.GetValue(), tt.wantValue)
			}
			if got.GetLength() != tt.wantLength {
				t.Errorf("UnmarshalBcdString().GetLength() = %v, want %v", got.GetLength(), tt.wantLength)
			}
		})
	}
}

func TestUnmarshalBcdString_Unwritten(t *testing.T) {
	// An unwritten field reads back as all-0xFF (seen as VuDataBlockCounter on
	// a real Gen1 driver card). It must yield no value, not fail the file.
	opts := UnmarshalOptions{}
	for _, input := range [][]byte{{0xff}, {0xff, 0xff}} {
		got, err := opts.UnmarshalBcdString(input)
		if err != nil {
			t.Errorf("UnmarshalBcdString(%x) unexpected error: %v", input, err)
		}
		if got != nil {
			t.Errorf("UnmarshalBcdString(%x) = %v, want nil", input, got)
		}
	}
	// Only a fully unwritten field qualifies; a stray 0xFF is still corrupt.
	if _, err := opts.UnmarshalBcdString([]byte{0xff, 0x12}); err == nil {
		t.Error("UnmarshalBcdString(ff12) expected error, got nil")
	}
}

func TestAppendBcdString(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		wantBytes []byte
	}{
		{
			name:      "valid BCD string",
			input:     []byte{0x12, 0x34},
			wantBytes: []byte{0x12, 0x34},
		},
		{
			name:      "single byte",
			input:     []byte{0x99},
			wantBytes: []byte{0x99},
		},
		{
			name:      "empty BCD string",
			input:     []byte{},
			wantBytes: []byte{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := MarshalOptions{}
			// Create BCD string from input
			var bcdString *ddv1.BcdString
			if len(tt.input) > 0 {
				unmarshalOpts := UnmarshalOptions{}
				var err error
				bcdString, err = unmarshalOpts.UnmarshalBcdString(tt.input)
				if err != nil {
					t.Fatalf("UnmarshalBcdString() unexpected error: %v", err)
				}
			}
			got, err := opts.MarshalBcdString(bcdString)
			if err != nil {
				t.Fatalf("AppendBcdString() unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.wantBytes, got); diff != "" {
				t.Errorf("AppendBcdString() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBcdStringRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{
			name:  "two bytes",
			input: []byte{0x12, 0x34},
		},
		{
			name:  "four bytes",
			input: []byte{0x20, 0x25, 0x09, 0x30},
		},
		{
			name:  "single byte",
			input: []byte{0x42},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Unmarshal
			unmarshalOpts := UnmarshalOptions{}
			bcdString, err := unmarshalOpts.UnmarshalBcdString(tt.input)
			if err != nil {
				t.Fatalf("UnmarshalBcdString() unexpected error: %v", err)
			}
			if bcdString == nil {
				t.Fatal("UnmarshalBcdString() returned nil")
			}
			// Marshal
			marshalOpts := MarshalOptions{}
			got, err := marshalOpts.MarshalBcdString(bcdString)
			if err != nil {
				t.Fatalf("AppendBcdString() unexpected error: %v", err)
			}
			// Verify round-trip
			if diff := cmp.Diff(tt.input, got); diff != "" {
				t.Errorf("Round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAppendBcdString_WithLength(t *testing.T) {
	tests := []struct {
		name      string
		value     int32
		length    int32
		wantBytes []byte
		wantErr   bool
	}{
		{
			name:      "value 5 in 1 byte",
			value:     5,
			length:    1,
			wantBytes: []byte{0x05},
		},
		{
			name:      "value 42 in 1 byte",
			value:     42,
			length:    1,
			wantBytes: []byte{0x42},
		},
		{
			name:      "value 123 in 2 bytes (with leading zero padding)",
			value:     123,
			length:    2,
			wantBytes: []byte{0x01, 0x23},
		},
		{
			name:      "value 1234 in 2 bytes",
			value:     1234,
			length:    2,
			wantBytes: []byte{0x12, 0x34},
		},
		{
			name:      "value 42 in 2 bytes (with zero padding)",
			value:     42,
			length:    2,
			wantBytes: []byte{0x00, 0x42},
		},
		{
			name:      "value 0 in 1 byte",
			value:     0,
			length:    1,
			wantBytes: []byte{0x00},
		},
		{
			name:      "value 9999 in 2 bytes (DailyPresenceCounter max)",
			value:     9999,
			length:    2,
			wantBytes: []byte{0x99, 0x99},
		},
		{
			name:    "value too large for length",
			value:   12345,
			length:  2,
			wantErr: true,
		},
		{
			name:    "negative value",
			value:   -1,
			length:  1,
			wantErr: true,
		},
		{
			name:    "zero length",
			value:   5,
			length:  0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := MarshalOptions{}
			bcdString := &ddv1.BcdString{}
			bcdString.SetValue(tt.value)
			bcdString.SetLength(tt.length)

			got, err := opts.MarshalBcdString(bcdString)
			if tt.wantErr {
				if err == nil {
					t.Errorf("AppendBcdString() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("AppendBcdString() unexpected error: %v", err)
			}

			if diff := cmp.Diff(tt.wantBytes, got); diff != "" {
				t.Errorf("AppendBcdString() mismatch (-want +got):\n%s", diff)
			}

			// Verify it can be decoded back to the same value
			decoded, err := decodeBCD(got)
			if err != nil {
				t.Fatalf("decodeBCD() failed: %v", err)
			}
			if int32(decoded) != tt.value {
				t.Errorf("Decoded value = %d, want %d", decoded, tt.value)
			}
		})
	}
}
