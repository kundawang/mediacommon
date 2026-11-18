package h264

import (
	"fmt"

	"github.com/bluenviron/mediacommon/v2/pkg/bits"
)

const (
	seiPayloadTypeUserDataRegistered = 4

	ituT35CountryCodeUS    = 0xB5
	ituT35ProviderCodeATSC = 0x0031
	a53UserDataTypeCC      = 0x03
	a53CCDataMarkerBits    = 0xFF
	a53CCDataBlockSize     = 3
	a53CCDataMaxCount      = 31
)

// CCData is a closed caption data block (cc_data) carried by an ATSC A/53
// user_data_registered_itu_t_t35 SEI message.
// Specification: ATSC A/53 Part 4, section 4.3
type CCData struct {
	// CCValid is the cc_valid bit. When false, the block does not carry
	// usable caption data.
	CCValid bool

	// CCType is the cc_type field.
	// Possible values:
	// - 0: CEA-608 field 1
	// - 1: CEA-608 field 2
	// - 2: CEA-708 channel packet data
	// - 3: CEA-708 channel packet start
	CCType uint8

	// Data is the cc_data_1 / cc_data_2 pair.
	Data [2]byte
}

// CCDataGroup is the set of cc_data blocks extracted from a single
// user_data_registered_itu_t_t35 SEI message.
type CCDataGroup struct {
	// CCData contains the cc_data blocks in their original order.
	CCData []CCData

	// PTS is the timestamp of the access unit the blocks belong to.
	PTS int64
}

func parseCCDataBlock(buf []byte) (CCData, error) {
	if len(buf) != a53CCDataBlockSize {
		return CCData{}, fmt.Errorf("cc_data block must be %d bytes", a53CCDataBlockSize)
	}

	pos := 0
	ccValid, err := bits.ReadFlag(buf, &pos)
	if err != nil {
		return CCData{}, err
	}

	ccType, err := bits.ReadBits(buf, &pos, 2)
	if err != nil {
		return CCData{}, err
	}

	_, err = bits.ReadBits(buf, &pos, 5) // reserved
	if err != nil {
		return CCData{}, err
	}

	return CCData{
		CCValid: ccValid,
		CCType:  uint8(ccType),
		Data: [2]byte{
			buf[1],
			buf[2],
		},
	}, nil
}

// parseGA94Payload parses the payload of a GA94 user data identifier
// (the bytes following the identifier).
func parseGA94Payload(payload []byte) ([]CCData, error) {
	// user_data_type_code
	if len(payload) < 1 {
		return nil, fmt.Errorf("GA94 payload is too short")
	}

	if payload[0] != a53UserDataTypeCC {
		// not a closed caption user data block (e.g. bar data)
		return nil, nil
	}

	if len(payload) < 2 {
		return nil, fmt.Errorf("GA94 payload is too short")
	}

	pos := 8
	// reserved
	_, err := bits.ReadBits(payload, &pos, 1)
	if err != nil {
		return nil, err
	}

	// process_cc_data_flag
	processCCData, err := bits.ReadFlag(payload, &pos)
	if err != nil {
		return nil, err
	}

	// zero bit
	zero, err := bits.ReadFlag(payload, &pos)
	if err != nil {
		return nil, err
	}
	if zero {
		return nil, fmt.Errorf("GA94 zero bit is not zero")
	}

	ccCount, err := bits.ReadBits(payload, &pos, 5)
	if err != nil {
		return nil, err
	}

	if !processCCData || ccCount == 0 {
		return nil, nil
	}

	if ccCount > a53CCDataMaxCount {
		return nil, fmt.Errorf("cc_count (%d) exceeds maximum allowed (%d)", ccCount, a53CCDataMaxCount)
	}

	// layout after the two header bytes: reserved (1 byte) +
	// cc_count cc_data blocks (3 bytes each) + marker_bits (1 byte)
	needed := 2 + 1 + int(ccCount)*a53CCDataBlockSize + 1
	if len(payload) < needed {
		return nil, fmt.Errorf("GA94 payload is too short for cc_count = %d", ccCount)
	}

	off := 3 // skip user_data_type_code, flags and the second reserved byte
	ret := make([]CCData, ccCount)
	for i := range ret {
		block, parseErr := parseCCDataBlock(payload[off : off+a53CCDataBlockSize])
		if parseErr != nil {
			return nil, parseErr
		}
		ret[i] = block
		off += a53CCDataBlockSize
	}

	if payload[off] != a53CCDataMarkerBits {
		return nil, fmt.Errorf("GA94 marker_bits must be 0xff, got 0x%02x", payload[off])
	}

	return ret, nil
}

// parseT35Payload parses a user_data_registered_itu_t_t35 SEI payload.
// It returns nil when the payload does not contain GA94 closed captions.
func parseT35Payload(payload []byte) ([]CCData, error) {
	// itu_t_t35_country_code
	if len(payload) < 1 {
		return nil, fmt.Errorf("T.35 payload is too short")
	}

	pos := 1
	if payload[0] == 0xFF {
		// itu_t_t35_country_code_extension_byte
		if len(payload) < 2 {
			return nil, fmt.Errorf("T.35 payload is too short")
		}
		pos++
	} else if payload[0] != ituT35CountryCodeUS {
		return nil, nil
	}

	// itu_t_t35_provider_code
	if len(payload) < pos+2 {
		return nil, fmt.Errorf("T.35 payload is too short")
	}

	providerCode := uint16(payload[pos])<<8 | uint16(payload[pos+1])
	pos += 2

	if providerCode != ituT35ProviderCodeATSC {
		return nil, nil
	}

	// ATSC_user_identifier
	if len(payload) < pos+4 {
		return nil, fmt.Errorf("T.35 payload is too short")
	}

	identifier := payload[pos : pos+4]
	pos += 4

	if string(identifier) != "GA94" {
		// other ATSC user data (e.g. DTG1 AFD) remains available to other parsers
		return nil, nil
	}

	return parseGA94Payload(payload[pos:])
}

// forEachSEIMessage iterates over the SEI messages contained in a NALU.
// The NALU header byte is expected to be present.
func forEachSEIMessage(nalu []byte, cb func(payloadType int, payload []byte) error) error {
	if len(nalu) < 2 {
		return fmt.Errorf("SEI NALU is too short")
	}

	rbsp := EmulationPreventionRemove(nalu[1:])
	pos := 0

	for pos < len(rbsp) {
		// RBSP stop bit (rbsp_trailing_bits)
		if rbsp[pos] == 0x80 && pos == len(rbsp)-1 {
			return nil
		}

		payloadType, next, ok := readSEIByteValue(rbsp, pos)
		if !ok {
			return fmt.Errorf("truncated SEI payload type")
		}
		pos = next

		payloadSize, next, ok := readSEIByteValue(rbsp, pos)
		if !ok {
			return fmt.Errorf("truncated SEI payload size")
		}
		pos = next

		if payloadSize > len(rbsp)-pos {
			return fmt.Errorf("SEI payload size (%d) exceeds remaining bytes (%d)", payloadSize, len(rbsp)-pos)
		}

		err := cb(payloadType, rbsp[pos:pos+payloadSize])
		if err != nil {
			return err
		}
		pos += payloadSize
	}

	return nil
}

// ExtractCCData extracts GA94 closed caption data (CEA-608 and CEA-708)
// from the user_data_registered_itu_t_t35 SEI messages of an access unit.
//
// The NALUs are only read: they are never modified. SEI messages that do
// not contain GA94 closed captions are ignored, so other user data remains
// available. The returned groups and their cc_data blocks are ordered as
// found in the access unit, and every group is tagged with pts.
func ExtractCCData(au [][]byte, pts int64) ([]CCDataGroup, error) {
	var groups []CCDataGroup

	for _, nalu := range au {
		if len(nalu) == 0 || NALUType(nalu[0]&0x1F) != NALUTypeSEI {
			continue
		}

		err := forEachSEIMessage(nalu, func(payloadType int, payload []byte) error {
			if payloadType != seiPayloadTypeUserDataRegistered {
				return nil
			}

			ccData, err := parseT35Payload(payload)
			if err != nil {
				return err
			}

			if len(ccData) != 0 {
				groups = append(groups, CCDataGroup{
					CCData: ccData,
					PTS:    pts,
				})
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return groups, nil
}
