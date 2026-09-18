package card

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	cardv1 "github.com/way-platform/tachograph-go/proto/gen/go/wayplatform/connect/tachograph/card/v1"
	ddv1 "github.com/way-platform/tachograph-go/proto/gen/go/wayplatform/connect/tachograph/dd/v1"
)

func TestFaults_Generation1(t *testing.T) {
	// Discover all matching hexdump files using type-safe enums
	hexdumpFiles, err := findHexdumpFiles(
		cardv1.ElementaryFileType_EF_FAULTS_DATA,
		ddv1.Generation_GENERATION_1,
		cardv1.ContentType_DATA,
	)
	if err != nil {
		t.Fatalf("Failed to discover hexdump files: %v", err)
	}
	if len(hexdumpFiles) == 0 {
		t.Fatal("No hexdump files found for EF_FAULTS_DATA GENERATION_1")
	}

	// Run subtest for each discovered file
	for _, hexdumpPath := range hexdumpFiles {
		// Use relative path from testdata as subtest name
		relPath := strings.TrimPrefix(hexdumpPath, "testdata/records/")
		testName := strings.TrimSuffix(relPath, ".hexdump")

		t.Run(testName, func(t *testing.T) {
			// Read hexdump
			data, err := readHexdump(hexdumpPath)
			if err != nil {
				t.Fatalf("Failed to read hexdump: %v", err)
			}

			// Unmarshal
			opts := UnmarshalOptions{}
			faults, err := opts.unmarshalFaultsData(data)
			if err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			// Golden JSON comparison
			goldenPath := goldenJSONPath(hexdumpPath)
			loadOrCreateGolden(t, faults, goldenPath)

			// Round-trip test
			marshalOpts := MarshalOptions{}
			marshaled, err := marshalOpts.MarshalFaultsData(faults)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}
			if diff := cmp.Diff(data, marshaled); diff != "" {
				t.Errorf("Binary round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFaults_Generation2(t *testing.T) {
	// Discover all matching hexdump files using type-safe enums
	hexdumpFiles, err := findHexdumpFiles(
		cardv1.ElementaryFileType_EF_FAULTS_DATA,
		ddv1.Generation_GENERATION_2,
		cardv1.ContentType_DATA,
	)
	if err != nil {
		t.Fatalf("Failed to discover hexdump files: %v", err)
	}
	if len(hexdumpFiles) == 0 {
		t.Fatal("No hexdump files found for EF_FAULTS_DATA GENERATION_2")
	}

	// Run subtest for each discovered file
	for _, hexdumpPath := range hexdumpFiles {
		// Use relative path from testdata as subtest name
		relPath := strings.TrimPrefix(hexdumpPath, "testdata/records/")
		testName := strings.TrimSuffix(relPath, ".hexdump")

		t.Run(testName, func(t *testing.T) {
			// Read hexdump
			data, err := readHexdump(hexdumpPath)
			if err != nil {
				t.Fatalf("Failed to read hexdump: %v", err)
			}

			// Unmarshal
			opts := UnmarshalOptions{}
			faults, err := opts.unmarshalFaultsData(data)
			if err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			// Golden JSON comparison
			goldenPath := goldenJSONPath(hexdumpPath)
			loadOrCreateGolden(t, faults, goldenPath)

			// Round-trip test
			marshalOpts := MarshalOptions{}
			marshaled, err := marshalOpts.MarshalFaultsData(faults)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}
			if diff := cmp.Diff(data, marshaled); diff != "" {
				t.Errorf("Binary round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// buildCardFaultRecord assembles one 24-byte CardFaultRecord carrying the given
// fault-type byte. The begin time is non-zero so the record takes the "valid"
// branch in unmarshalFaultsData.
func buildCardFaultRecord(faultType byte) []byte {
	rec := make([]byte, cardFaultRecordSize)
	rec[0] = faultType
	binary.BigEndian.PutUint32(rec[1:5], 0x4D2A0000) // fault begin time
	binary.BigEndian.PutUint32(rec[5:9], 0x4D2A0E10) // fault end time
	rec[9] = 0x01                                    // nation
	rec[10] = 0x01                                   // code page
	copy(rec[11:24], "ABC1234      ")
	return rec
}

// TestFaults_UnrecordedFaultTypeIsPreserved covers a real driver card that the
// decoder rejected outright: one fault record held 0xFF in its fault-type byte,
// which is what a tachograph writes into a field it never recorded. 0xFF has no
// protocol_enum_value, so the whole 67KB card failed to parse over a field
// nothing reads. The record is now preserved verbatim instead.
func TestFaults_UnrecordedFaultTypeIsPreserved(t *testing.T) {
	opts := UnmarshalOptions{}

	// Control: a mappable fault type still parses semantically.
	known := buildCardFaultRecord(0x00) // GENERAL_NO_FURTHER_DETAILS
	faults, err := opts.unmarshalFaultsData(known)
	if err != nil {
		t.Fatalf("Unmarshal of a known fault type failed: %v", err)
	}
	if recs := faults.GetFaults(); len(recs) != 1 || !recs[0].GetValid() {
		t.Fatalf("Known fault type: want 1 valid record, got %d", len(recs))
	}

	unrecorded := buildCardFaultRecord(0xFF)
	faults, err = opts.unmarshalFaultsData(unrecorded)
	if err != nil {
		t.Fatalf("Unmarshal of an unrecorded fault type failed: %v", err)
	}
	recs := faults.GetFaults()
	if len(recs) != 1 {
		t.Fatalf("want 1 record, got %d", len(recs))
	}
	if recs[0].GetValid() {
		t.Error("want the unparseable record marked not valid")
	}
	if diff := cmp.Diff(unrecorded, recs[0].GetRawData()); diff != "" {
		t.Errorf("Raw data not preserved (-want +got):\n%s", diff)
	}

	marshaled, err := (MarshalOptions{}).MarshalFaultsData(faults)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if diff := cmp.Diff(unrecorded, marshaled); diff != "" {
		t.Errorf("Binary round-trip mismatch (-want +got):\n%s", diff)
	}
}
