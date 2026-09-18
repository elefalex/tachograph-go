package card

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	cardv1 "github.com/way-platform/tachograph-go/proto/gen/go/wayplatform/connect/tachograph/card/v1"
	ddv1 "github.com/way-platform/tachograph-go/proto/gen/go/wayplatform/connect/tachograph/dd/v1"
)

func TestEvents_Generation1(t *testing.T) {
	// Discover all matching hexdump files using type-safe enums
	hexdumpFiles, err := findHexdumpFiles(
		cardv1.ElementaryFileType_EF_EVENTS_DATA,
		ddv1.Generation_GENERATION_1,
		cardv1.ContentType_DATA,
	)
	if err != nil {
		t.Fatalf("Failed to discover hexdump files: %v", err)
	}
	if len(hexdumpFiles) == 0 {
		t.Fatal("No hexdump files found for EF_EVENTS_DATA GENERATION_1")
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
			events, err := opts.unmarshalEventsData(data)
			if err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			// Golden JSON comparison
			goldenPath := goldenJSONPath(hexdumpPath)
			loadOrCreateGolden(t, events, goldenPath)

			// Round-trip test
			marshalOpts := MarshalOptions{}
			marshaled, err := marshalOpts.MarshalEventsData(events)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}
			if diff := cmp.Diff(data, marshaled); diff != "" {
				t.Errorf("Binary round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEvents_Generation2(t *testing.T) {
	// Discover all matching hexdump files using type-safe enums
	hexdumpFiles, err := findHexdumpFiles(
		cardv1.ElementaryFileType_EF_EVENTS_DATA,
		ddv1.Generation_GENERATION_2,
		cardv1.ContentType_DATA,
	)
	if err != nil {
		t.Fatalf("Failed to discover hexdump files: %v", err)
	}
	if len(hexdumpFiles) == 0 {
		t.Fatal("No hexdump files found for EF_EVENTS_DATA GENERATION_2")
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
			events, err := opts.unmarshalEventsData(data)
			if err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			// Golden JSON comparison
			goldenPath := goldenJSONPath(hexdumpPath)
			loadOrCreateGolden(t, events, goldenPath)

			// Round-trip test
			marshalOpts := MarshalOptions{}
			marshaled, err := marshalOpts.MarshalEventsData(events)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}
			if diff := cmp.Diff(data, marshaled); diff != "" {
				t.Errorf("Binary round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestEvents_UnrecordedEventTypeIsPreserved is the event-record sibling of
// TestFaults_UnrecordedFaultTypeIsPreserved: event and fault records share the
// same 24-byte layout and the same EventFaultType enum, so an unrecorded 0xFF
// type byte used to fail the whole card here too.
func TestEvents_UnrecordedEventTypeIsPreserved(t *testing.T) {
	opts := UnmarshalOptions{}

	// Control: a mappable event type still parses semantically.
	known := buildCardFaultRecord(0x00) // GENERAL_NO_FURTHER_DETAILS
	events, err := opts.unmarshalEventsData(known)
	if err != nil {
		t.Fatalf("Unmarshal of a known event type failed: %v", err)
	}
	if recs := events.GetEvents(); len(recs) != 1 || !recs[0].GetValid() {
		t.Fatalf("Known event type: want 1 valid record, got %d", len(recs))
	}

	unrecorded := buildCardFaultRecord(0xFF)
	events, err = opts.unmarshalEventsData(unrecorded)
	if err != nil {
		t.Fatalf("Unmarshal of an unrecorded event type failed: %v", err)
	}
	recs := events.GetEvents()
	if len(recs) != 1 {
		t.Fatalf("want 1 record, got %d", len(recs))
	}
	if recs[0].GetValid() {
		t.Error("want the unparseable record marked not valid")
	}
	if diff := cmp.Diff(unrecorded, recs[0].GetRawData()); diff != "" {
		t.Errorf("Raw data not preserved (-want +got):\n%s", diff)
	}

	marshaled, err := (MarshalOptions{}).MarshalEventsData(events)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if diff := cmp.Diff(unrecorded, marshaled); diff != "" {
		t.Errorf("Binary round-trip mismatch (-want +got):\n%s", diff)
	}
}
