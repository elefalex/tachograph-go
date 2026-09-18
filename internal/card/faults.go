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

// unmarshalFaultsData parses the binary data for an EF_Faults_Data record.
//
// The data type `CardFaultData` is specified in the Data Dictionary, Section 2.22.
//
// ASN.1 Definition:
//
//	CardFaultData ::= SEQUENCE OF CardFaultRecord
//
//	CardFaultRecord ::= SEQUENCE {
//	    faultType                   EventFaultType,                     -- 1 byte
//	    faultBeginTime              TimeReal,                         -- 4 bytes
//	    faultEndTime                TimeReal,                         -- 4 bytes
//	    faultVehicleRegistration    VehicleRegistrationIdentification -- 15 bytes
//	}
const (
	// CardFaultRecord size (24 bytes total)
	cardFaultRecordSize = 24
)

// splitCardFaultRecord returns a SplitFunc that splits data into 24-byte fault records
func splitCardFaultRecord(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if len(data) < cardFaultRecordSize {
		if atEOF {
			return 0, nil, nil // No more complete records, but not an error
		}
		return 0, nil, nil // Need more data
	}

	return cardFaultRecordSize, data[:cardFaultRecordSize], nil
}

func (opts UnmarshalOptions) unmarshalFaultsData(data []byte) (*cardv1.FaultsData, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Split(splitCardFaultRecord)

	var records []*cardv1.FaultsData_Record
	for scanner.Scan() {
		recordData := scanner.Bytes()
		// Check if this is a valid record by examining the fault begin time (first 4 bytes after fault type)
		// Fault type is 1 byte, so fault begin time starts at byte 1
		faultBeginTime := binary.BigEndian.Uint32(recordData[1:5])

		rec := &cardv1.FaultsData_Record{}

		if faultBeginTime == 0 {
			// Non-valid record: preserve original bytes
			rec.SetValid(false)
			rec.SetRawData(recordData)
		} else {
			// Valid record: parse semantic data
			rec.SetValid(true)
			if err := opts.unmarshalFaultRecord(recordData, rec); err != nil {
				// One unparseable record must not lose the whole card.
				// Tachograph data writes 0xFF into fields that were never
				// recorded, and those bytes reach enums with no mapping for
				// them. Preserve the record verbatim instead, exactly as the
				// non-valid branch above does, which also keeps the binary
				// round-trip byte-exact.
				rec = &cardv1.FaultsData_Record{}
				rec.SetValid(false)
				rec.SetRawData(recordData)
			}
		}

		records = append(records, rec)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Use simplified schema with single faults array in chronological order
	var fd cardv1.FaultsData
	fd.SetFaults(records)
	return &fd, nil
}

// UnmarshalFaultRecord parses a single fault record.
//
// The data type `CardFaultRecord` is specified in the Data Dictionary, Section 2.22.
//
// ASN.1 Definition:
//
//	CardFaultRecord ::= SEQUENCE {
//	    faultType                   EventFaultType,                     -- 1 byte
//	    faultBeginTime              TimeReal,                         -- 4 bytes
//	    faultEndTime                TimeReal,                         -- 4 bytes
//	    faultVehicleRegistration    VehicleRegistrationIdentification -- 15 bytes
//	}
func (opts UnmarshalOptions) unmarshalFaultRecord(data []byte, rec *cardv1.FaultsData_Record) error {
	const (
		lenFaultType                = 1
		lenFaultBeginTime           = 4
		lenFaultEndTime             = 4
		lenFaultVehicleRegistration = 15
		lenCardFaultRecord          = lenFaultType + lenFaultBeginTime + lenFaultEndTime + lenFaultVehicleRegistration
	)

	if len(data) < lenCardFaultRecord {
		return fmt.Errorf("insufficient data for fault record: got %d bytes, need %d", len(data), lenCardFaultRecord)
	}

	offset := 0

	// Read fault type (1 byte) and convert using generic enum helper
	if offset+1 > len(data) {
		return fmt.Errorf("insufficient data for fault type")
	}
	if faultTypeEnum, err := dd.UnmarshalEnum[ddv1.EventFaultType](data[offset]); err == nil {
		rec.SetFaultType(faultTypeEnum)
	} else {
		return fmt.Errorf("invalid fault type: %w", err)
	}
	offset++

	// Read fault begin time (4 bytes)
	if offset+4 > len(data) {
		return fmt.Errorf("insufficient data for fault begin time")
	}
	faultBeginTime, err := opts.UnmarshalTimeReal(data[offset : offset+4])
	if err != nil {
		return fmt.Errorf("failed to parse fault begin time: %w", err)
	}
	rec.SetFaultBeginTime(faultBeginTime)
	offset += 4

	// Read fault end time (4 bytes)
	if offset+4 > len(data) {
		return fmt.Errorf("insufficient data for fault end time")
	}
	faultEndTime, err := opts.UnmarshalTimeReal(data[offset : offset+4])
	if err != nil {
		return fmt.Errorf("failed to parse fault end time: %w", err)
	}
	rec.SetFaultEndTime(faultEndTime)
	offset += 4

	// Read vehicle registration (15 bytes: 1 byte nation + 14 bytes number)
	if offset+15 > len(data) {
		return fmt.Errorf("insufficient data for vehicle registration")
	}
	vehicleReg, err := opts.UnmarshalVehicleRegistration(data[offset : offset+15])
	if err != nil {
		return fmt.Errorf("failed to parse vehicle registration: %w", err)
	}
	// offset += 15 // Not needed as this is the last field
	rec.SetFaultVehicleRegistration(vehicleReg)
	return nil
}

// MarshalFaultsData marshals the binary representation of FaultData.
//
// The data type `CardFaultData` is specified in the Data Dictionary, Section 2.22.
//
// ASN.1 Definition:
//
//	CardFaultData ::= SEQUENCE OF CardFaultRecord
//
//	CardFaultRecord ::= SEQUENCE {
//	    faultType                         EventFaultType,                     -- 1 byte
//	    faultBeginTime                    TimeReal,                         -- 4 bytes
//	    faultEndTime                      TimeReal,                         -- 4 bytes
//	    faultVehicleRegistration          VehicleRegistrationIdentification -- 15 bytes
//	}
func (opts MarshalOptions) MarshalFaultsData(data *cardv1.FaultsData) ([]byte, error) {
	if data == nil {
		return nil, nil
	}

	var dst []byte

	// Process faults in their chronological order
	for _, r := range data.GetFaults() {
		recordBytes, err := opts.MarshalFaultRecord(r)
		if err != nil {
			return nil, err
		}
		dst = append(dst, recordBytes...)
	}
	return dst, nil
}

// MarshalFaultRecord marshals a single fault record.
//
// The data type `CardFaultRecord` is specified in the Data Dictionary, Section 2.22.
//
// ASN.1 Definition:
//
//	CardFaultRecord ::= SEQUENCE {
//	    faultType                         EventFaultType,                     -- 1 byte
//	    faultBeginTime                    TimeReal,                         -- 4 bytes
//	    faultEndTime                      TimeReal,                         -- 4 bytes
//	    faultVehicleRegistration          VehicleRegistrationIdentification -- 15 bytes
//	}
func (opts MarshalOptions) MarshalFaultRecord(record *cardv1.FaultsData_Record) ([]byte, error) {
	if record == nil {
		return nil, nil
	}

	if !record.GetValid() {
		return record.GetRawData(), nil
	}

	var dst []byte

	protocolValue, _ := dd.MarshalEnum(record.GetFaultType())
	dst = append(dst, protocolValue)

	beginTimeBytes, err := opts.MarshalTimeReal(record.GetFaultBeginTime())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal fault begin time: %w", err)
	}
	dst = append(dst, beginTimeBytes...)

	endTimeBytes, err := opts.MarshalTimeReal(record.GetFaultEndTime())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal fault end time: %w", err)
	}
	dst = append(dst, endTimeBytes...)

	vehicleRegBytes, err := opts.MarshalVehicleRegistration(record.GetFaultVehicleRegistration())
	if err != nil {
		return nil, err
	}
	dst = append(dst, vehicleRegBytes...)

	return dst, nil
}

// anonymizeFaultsData creates an anonymized copy of FaultsData,
// replacing sensitive information with static, deterministic test values.
func (opts AnonymizeOptions) anonymizeFaultsData(faults *cardv1.FaultsData) *cardv1.FaultsData {
	if faults == nil {
		return nil
	}

	anonymized := &cardv1.FaultsData{}

	// Create DD anonymize options
	ddOpts := dd.AnonymizeOptions{
		PreserveDistanceAndTrips: opts.PreserveDistanceAndTrips,
		PreserveTimestamps:       opts.PreserveTimestamps,
	}

	// Base timestamp for anonymization: 2020-01-01 00:00:00 UTC (epoch: 1577836800)
	baseEpoch := int64(1577836800)

	var anonymizedFaults []*cardv1.FaultsData_Record
	for i, fault := range faults.GetFaults() {
		anonymizedFault := &cardv1.FaultsData_Record{}

		// Preserve valid flag
		anonymizedFault.SetValid(fault.GetValid())

		if fault.GetValid() {
			// Preserve fault type (not sensitive, categorical)
			anonymizedFault.SetFaultType(fault.GetFaultType())

			// Use incrementing timestamps based on index (1 hour apart)
			beginTime := &timestamppb.Timestamp{Seconds: baseEpoch + int64(i)*3600}
			endTime := &timestamppb.Timestamp{Seconds: baseEpoch + int64(i)*3600 + 1800} // 30 mins later
			anonymizedFault.SetFaultBeginTime(beginTime)
			anonymizedFault.SetFaultEndTime(endTime)

			// Anonymize vehicle registration
			if vehicleReg := fault.GetFaultVehicleRegistration(); vehicleReg != nil {
				anonymizedFault.SetFaultVehicleRegistration(ddOpts.AnonymizeVehicleRegistrationIdentification(vehicleReg))
			}

			// Regenerate raw_data for binary fidelity
			marshalOpts := MarshalOptions{}
			rawData, err := marshalOpts.MarshalFaultRecord(anonymizedFault)
			if err == nil {
				anonymizedFault.SetRawData(rawData)
			}
		} else {
			// Preserve invalid records as-is
			anonymizedFault.SetRawData(fault.GetRawData())
		}

		anonymizedFaults = append(anonymizedFaults, anonymizedFault)
	}

	anonymized.SetFaults(anonymizedFaults)

	// Signature field left unset (nil) - TLV marshaller will omit the signature block

	return anonymized
}
