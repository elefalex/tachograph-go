package card

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/way-platform/tachograph-go/internal/dd"

	cardv1 "github.com/way-platform/tachograph-go/proto/gen/go/wayplatform/connect/tachograph/card/v1"
	ddv1 "github.com/way-platform/tachograph-go/proto/gen/go/wayplatform/connect/tachograph/dd/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// unmarshalEventsData unmarshals events data from a card EF.
//
// The data type `CardEventData` is specified in the Data Dictionary, Section 2.19.
//
// ASN.1 Definition:
//
//	CardEventData ::= SEQUENCE OF CardEventRecord
//
//	CardEventRecord ::= SEQUENCE {
//	    eventType                   EventFaultType,                     -- 1 byte
//	    eventBeginTime              TimeReal,                         -- 4 bytes
//	    eventEndTime                TimeReal,                         -- 4 bytes
//	    eventVehicleRegistration    VehicleRegistrationIdentification -- 15 bytes
//	}
const (
	// CardEventRecord size (24 bytes total)
	cardEventRecordSize = 24
)

// splitCardEventRecord returns a SplitFunc that splits data into 24-byte event records
func splitCardEventRecord(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if len(data) < cardEventRecordSize {
		if atEOF {
			return 0, nil, nil // No more complete records, but not an error
		}
		return 0, nil, nil // Need more data
	}

	return cardEventRecordSize, data[:cardEventRecordSize], nil
}

func (opts UnmarshalOptions) unmarshalEventsData(data []byte) (*cardv1.EventsData, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Split(splitCardEventRecord)

	var records []*cardv1.EventsData_Record
	for scanner.Scan() {
		recordData := scanner.Bytes()
		// Check if this is a valid record by examining the event begin time (first 4 bytes after event type)
		// Event type is 1 byte, so event begin time starts at byte 1
		eventBeginTime := binary.BigEndian.Uint32(recordData[1:5])
		if eventBeginTime == 0 {
			// Non-valid record: preserve original bytes
			rec := &cardv1.EventsData_Record{}
			rec.SetValid(false)
			rec.SetRawData(recordData)
			records = append(records, rec)
		} else {
			// Valid record: parse semantic data
			rec, err := opts.unmarshalEventRecord(recordData)
			if err != nil {
				// One unparseable record must not lose the whole card.
				// Tachograph data writes 0xFF into fields that were never
				// recorded, and those bytes reach enums with no mapping for
				// them. Preserve the record verbatim instead, exactly as the
				// non-valid branch above does, which also keeps the binary
				// round-trip byte-exact.
				rec = &cardv1.EventsData_Record{}
				rec.SetValid(false)
				rec.SetRawData(recordData)
			} else {
				rec.SetValid(true)
			}
			records = append(records, rec)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Use simplified schema with single events array in chronological order
	var ed cardv1.EventsData
	ed.SetEvents(records)
	return &ed, nil
}

// unmarshalEventRecord parses a single event record.
//
// The data type `CardEventRecord` is specified in the Data Dictionary, Section 2.20.
//
// ASN.1 Definition:
//
//	CardEventRecord ::= SEQUENCE {
//	    eventType                   EventFaultType,                     -- 1 byte
//	    eventBeginTime              TimeReal,                         -- 4 bytes
//	    eventEndTime                TimeReal,                         -- 4 bytes
//	    eventVehicleRegistration    VehicleRegistrationIdentification -- 15 bytes
//	}
func (opts UnmarshalOptions) unmarshalEventRecord(data []byte) (*cardv1.EventsData_Record, error) {
	const (
		lenEventType                = 1
		lenEventBeginTime           = 4
		lenEventEndTime             = 4
		lenEventVehicleRegistration = 15
		lenCardEventRecord          = lenEventType + lenEventBeginTime + lenEventEndTime + lenEventVehicleRegistration
	)

	if len(data) < lenCardEventRecord {
		return nil, fmt.Errorf("insufficient data for event record: got %d bytes, need %d", len(data), lenCardEventRecord)
	}

	var rec cardv1.EventsData_Record
	offset := 0

	// Read event type (1 byte) and convert using generic enum helper
	if offset+1 > len(data) {
		return nil, fmt.Errorf("insufficient data for event type")
	}
	if eventTypeEnum, err := dd.UnmarshalEnum[ddv1.EventFaultType](data[offset]); err == nil {
		rec.SetEventType(eventTypeEnum)
	} else {
		return nil, fmt.Errorf("invalid event type: %w", err)
	}
	offset++

	// Read event begin time (4 bytes)
	if offset+4 > len(data) {
		return nil, fmt.Errorf("insufficient data for event begin time")
	}
	eventBeginTime, err := opts.UnmarshalTimeReal(data[offset : offset+4])
	if err != nil {
		return nil, fmt.Errorf("failed to parse event begin time: %w", err)
	}
	rec.SetEventBeginTime(eventBeginTime)
	offset += 4

	// Read event end time (4 bytes)
	if offset+4 > len(data) {
		return nil, fmt.Errorf("insufficient data for event end time")
	}
	eventEndTime, err := opts.UnmarshalTimeReal(data[offset : offset+4])
	if err != nil {
		return nil, fmt.Errorf("failed to parse event end time: %w", err)
	}
	rec.SetEventEndTime(eventEndTime)
	offset += 4

	// Read vehicle registration (15 bytes: 1 byte nation + 14 bytes number)
	if offset+15 > len(data) {
		return nil, fmt.Errorf("insufficient data for vehicle registration")
	}
	vehicleReg, err := opts.UnmarshalVehicleRegistration(data[offset : offset+15])
	if err != nil {
		return nil, fmt.Errorf("failed to parse vehicle registration: %w", err)
	}
	// offset += 15 // Not needed as this is the last field
	rec.SetEventVehicleRegistration(vehicleReg)
	return &rec, nil
}

// MarshalEventsData marshals the binary representation of EventData.
//
// The data type `CardEventData` is specified in the Data Dictionary, Section 2.19.
//
// ASN.1 Definition:
//
//	CardEventData ::= SEQUENCE OF CardEventRecord
//
//	CardEventRecord ::= SEQUENCE {
//	    eventType                        EventFaultType,                     -- 1 byte
//	    eventBeginTime                   TimeReal,                         -- 4 bytes
//	    eventEndTime                     TimeReal,                         -- 4 bytes
//	    eventVehicleRegistration         VehicleRegistrationIdentification -- 15 bytes
//	}
func (opts MarshalOptions) MarshalEventsData(data *cardv1.EventsData) ([]byte, error) {
	if data == nil {
		return nil, nil
	}

	var dst []byte

	// Process events in their chronological order
	for _, r := range data.GetEvents() {
		recordBytes, err := opts.MarshalEventRecord(r)
		if err != nil {
			return nil, err
		}
		dst = append(dst, recordBytes...)
	}
	return dst, nil
}

// MarshalEventRecord marshals a single event record.
//
// The data type `CardEventRecord` is specified in the Data Dictionary, Section 2.20.
//
// ASN.1 Definition:
//
//	CardEventRecord ::= SEQUENCE {
//	    eventType                        EventFaultType,                     -- 1 byte
//	    eventBeginTime                   TimeReal,                         -- 4 bytes
//	    eventEndTime                     TimeReal,                         -- 4 bytes
//	    eventVehicleRegistration         VehicleRegistrationIdentification -- 15 bytes
//	}
func (opts MarshalOptions) MarshalEventRecord(record *cardv1.EventsData_Record) ([]byte, error) {
	if record == nil {
		return nil, nil
	}

	if !record.GetValid() {
		return record.GetRawData(), nil
	}

	var dst []byte

	protocolValue, _ := dd.MarshalEnum(record.GetEventType())
	dst = append(dst, protocolValue)

	beginTimeBytes, err := opts.MarshalTimeReal(record.GetEventBeginTime())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event begin time: %w", err)
	}
	dst = append(dst, beginTimeBytes...)

	endTimeBytes, err := opts.MarshalTimeReal(record.GetEventEndTime())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event end time: %w", err)
	}
	dst = append(dst, endTimeBytes...)

	vehicleRegBytes, err := opts.MarshalVehicleRegistration(record.GetEventVehicleRegistration())
	if err != nil {
		return nil, err
	}
	dst = append(dst, vehicleRegBytes...)

	return dst, nil
}

// anonymizeEventsData creates an anonymized copy of EventsData,
// replacing sensitive information with static, deterministic test values.
func (opts AnonymizeOptions) anonymizeEventsData(events *cardv1.EventsData) *cardv1.EventsData {
	if events == nil {
		return nil
	}

	anonymized := &cardv1.EventsData{}

	// Create DD anonymize options
	ddOpts := dd.AnonymizeOptions{
		PreserveDistanceAndTrips: opts.PreserveDistanceAndTrips,
		PreserveTimestamps:       opts.PreserveTimestamps,
	}

	// Base timestamp for anonymization: 2020-01-01 00:00:00 UTC (epoch: 1577836800)
	baseEpoch := int64(1577836800)

	var anonymizedEvents []*cardv1.EventsData_Record
	for i, event := range events.GetEvents() {
		anonymizedEvent := &cardv1.EventsData_Record{}

		// Preserve valid flag
		anonymizedEvent.SetValid(event.GetValid())

		if event.GetValid() {
			// Preserve event type (not sensitive, categorical)
			anonymizedEvent.SetEventType(event.GetEventType())

			// Use incrementing timestamps based on index (1 hour apart)
			beginTime := &timestamppb.Timestamp{Seconds: baseEpoch + int64(i)*3600}
			endTime := &timestamppb.Timestamp{Seconds: baseEpoch + int64(i)*3600 + 1800} // 30 mins later
			anonymizedEvent.SetEventBeginTime(beginTime)
			anonymizedEvent.SetEventEndTime(endTime)

			// Anonymize vehicle registration
			if vehicleReg := event.GetEventVehicleRegistration(); vehicleReg != nil {
				anonymizedEvent.SetEventVehicleRegistration(ddOpts.AnonymizeVehicleRegistrationIdentification(vehicleReg))
			}

			// Regenerate raw_data for binary fidelity
			marshalOpts := MarshalOptions{}
			rawData, err := marshalOpts.MarshalEventRecord(anonymizedEvent)
			if err == nil {
				anonymizedEvent.SetRawData(rawData)
			}
		} else {
			// Preserve invalid records as-is
			anonymizedEvent.SetRawData(event.GetRawData())
		}

		anonymizedEvents = append(anonymizedEvents, anonymizedEvent)
	}

	anonymized.SetEvents(anonymizedEvents)

	// Signature field left unset (nil) - TLV marshaller will omit the signature block

	return anonymized
}
