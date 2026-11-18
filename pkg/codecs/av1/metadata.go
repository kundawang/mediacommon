package av1

import (
	"fmt"

	"github.com/bluenviron/mediacommon/v2/pkg/bits"
)

// MetadataType is a metadata_type value of a Metadata OBU.
// Specification: AV1 Bitstream & Decoding Process, section 6.7.2
type MetadataType uint32

const (
	// MetadataTypeHDR_CLL is the HDR Content Light Level Information.
	MetadataTypeHDR_CLL MetadataType = 1 //nolint:revive

	// MetadataTypeHDR_MDCV is the HDR Mastering Display Color Volume.
	MetadataTypeHDR_MDCV MetadataType = 2 //nolint:revive
)

// Metadata_HDRContentLightLevel is the hdr_content_light_level_info() struct.
// Specification: AV1 Bitstream & Decoding Process, section 6.7.3
type Metadata_HDRContentLightLevel struct { //nolint:revive
	MaxContentLightLevel      uint16
	MaxFrameAverageLightLevel uint16
}

// Metadata_HDRMasteringDisplayColorVolume is the hdr_mastering_display_color_volume_info() struct.
// Specification: AV1 Bitstream & Decoding Process, section 6.7.4
type Metadata_HDRMasteringDisplayColorVolume struct { //nolint:revive
	PrimaryRChromaticityX   uint16
	PrimaryRChromaticityY   uint16
	PrimaryGChromaticityX   uint16
	PrimaryGChromaticityY   uint16
	PrimaryBChromaticityX   uint16
	PrimaryBChromaticityY   uint16
	WhitePointChromaticityX uint16
	WhitePointChromaticityY uint16
	MaxMasteringLuminance   uint32
	MinMasteringLuminance   uint32
}

// Metadata is a AV1 Metadata OBU.
// Specification: AV1 Bitstream & Decoding Process, section 6.7.1
type Metadata struct {
	Type MetadataType

	// HDRContentLightLevel is set when Type is MetadataTypeHDR_CLL.
	HDRContentLightLevel *Metadata_HDRContentLightLevel

	// HDRMasteringDisplayColorVolume is set when Type is MetadataTypeHDR_MDCV.
	HDRMasteringDisplayColorVolume *Metadata_HDRMasteringDisplayColorVolume
}

// Unmarshal decodes a Metadata OBU.
// The buffer can contain either a full OBU (header, size and payload)
// or an OBU without the size field.
func (m *Metadata) Unmarshal(buf []byte) error {
	var oh OBUHeader
	err := oh.Unmarshal(buf)
	if err != nil {
		return err
	}
	if oh.Type != OBUTypeMetadata {
		return fmt.Errorf("OBU type is not Metadata (%d)", oh.Type)
	}
	buf = buf[1:]

	if oh.HasSize {
		var size LEB128
		var n int
		n, err = size.Unmarshal(buf)
		if err != nil {
			return err
		}

		buf = buf[n:]
		if len(buf) != int(size) {
			return fmt.Errorf("wrong buffer size: expected %d, got %d", size, len(buf))
		}
	}

	var typ LEB128
	n, err := typ.Unmarshal(buf)
	if err != nil {
		return err
	}
	buf = buf[n:]
	m.Type = MetadataType(typ)

	m.HDRContentLightLevel = nil
	m.HDRMasteringDisplayColorVolume = nil

	switch m.Type {
	case MetadataTypeHDR_CLL:
		err = bits.HasSpace(buf, 0, 32)
		if err != nil {
			return fmt.Errorf("HDR_CLL payload is truncated: %w", err)
		}
		if len(buf) != 4 {
			return fmt.Errorf("HDR_CLL has invalid length: expected 4, got %d", len(buf))
		}

		pos := 0
		m.HDRContentLightLevel = &Metadata_HDRContentLightLevel{
			MaxContentLightLevel:      uint16(bits.ReadBitsUnsafe(buf, &pos, 16)),
			MaxFrameAverageLightLevel: uint16(bits.ReadBitsUnsafe(buf, &pos, 16)),
		}

	case MetadataTypeHDR_MDCV:
		err = bits.HasSpace(buf, 0, 24*8)
		if err != nil {
			return fmt.Errorf("HDR_MDCV payload is truncated: %w", err)
		}
		if len(buf) != 24 {
			return fmt.Errorf("HDR_MDCV has invalid length: expected 24, got %d", len(buf))
		}

		pos := 0
		m.HDRMasteringDisplayColorVolume = &Metadata_HDRMasteringDisplayColorVolume{}
		v := m.HDRMasteringDisplayColorVolume
		v.PrimaryRChromaticityX = uint16(bits.ReadBitsUnsafe(buf, &pos, 16))
		v.PrimaryRChromaticityY = uint16(bits.ReadBitsUnsafe(buf, &pos, 16))
		v.PrimaryGChromaticityX = uint16(bits.ReadBitsUnsafe(buf, &pos, 16))
		v.PrimaryGChromaticityY = uint16(bits.ReadBitsUnsafe(buf, &pos, 16))
		v.PrimaryBChromaticityX = uint16(bits.ReadBitsUnsafe(buf, &pos, 16))
		v.PrimaryBChromaticityY = uint16(bits.ReadBitsUnsafe(buf, &pos, 16))
		v.WhitePointChromaticityX = uint16(bits.ReadBitsUnsafe(buf, &pos, 16))
		v.WhitePointChromaticityY = uint16(bits.ReadBitsUnsafe(buf, &pos, 16))
		v.MaxMasteringLuminance = uint32(bits.ReadBitsUnsafe(buf, &pos, 32))
		v.MinMasteringLuminance = uint32(bits.ReadBitsUnsafe(buf, &pos, 32))

	default:
		return fmt.Errorf("unknown metadata_type: %d", m.Type)
	}

	return nil
}

func (m Metadata) marshalPayload() ([]byte, error) {
	switch m.Type {
	case MetadataTypeHDR_CLL:
		if m.HDRContentLightLevel == nil {
			return nil, fmt.Errorf("HDRContentLightLevel must be set for metadata_type %d", m.Type)
		}
		if m.HDRMasteringDisplayColorVolume != nil {
			return nil, fmt.Errorf("HDRMasteringDisplayColorVolume must be nil for metadata_type %d", m.Type)
		}

		payload := make([]byte, LEB128(m.Type).MarshalSize()+4)
		n := LEB128(m.Type).MarshalTo(payload)
		pos := n * 8
		v := m.HDRContentLightLevel
		bits.WriteBitsUnsafe(payload, &pos, uint64(v.MaxContentLightLevel), 16)
		bits.WriteBitsUnsafe(payload, &pos, uint64(v.MaxFrameAverageLightLevel), 16)
		return payload, nil

	case MetadataTypeHDR_MDCV:
		if m.HDRMasteringDisplayColorVolume == nil {
			return nil, fmt.Errorf("HDRMasteringDisplayColorVolume must be set for metadata_type %d", m.Type)
		}
		if m.HDRContentLightLevel != nil {
			return nil, fmt.Errorf("HDRContentLightLevel must be nil for metadata_type %d", m.Type)
		}

		payload := make([]byte, LEB128(m.Type).MarshalSize()+24)
		n := LEB128(m.Type).MarshalTo(payload)
		pos := n * 8
		v := m.HDRMasteringDisplayColorVolume
		bits.WriteBitsUnsafe(payload, &pos, uint64(v.PrimaryRChromaticityX), 16)
		bits.WriteBitsUnsafe(payload, &pos, uint64(v.PrimaryRChromaticityY), 16)
		bits.WriteBitsUnsafe(payload, &pos, uint64(v.PrimaryGChromaticityX), 16)
		bits.WriteBitsUnsafe(payload, &pos, uint64(v.PrimaryGChromaticityY), 16)
		bits.WriteBitsUnsafe(payload, &pos, uint64(v.PrimaryBChromaticityX), 16)
		bits.WriteBitsUnsafe(payload, &pos, uint64(v.PrimaryBChromaticityY), 16)
		bits.WriteBitsUnsafe(payload, &pos, uint64(v.WhitePointChromaticityX), 16)
		bits.WriteBitsUnsafe(payload, &pos, uint64(v.WhitePointChromaticityY), 16)
		bits.WriteBitsUnsafe(payload, &pos, uint64(v.MaxMasteringLuminance), 32)
		bits.WriteBitsUnsafe(payload, &pos, uint64(v.MinMasteringLuminance), 32)
		return payload, nil

	default:
		return nil, fmt.Errorf("unknown metadata_type: %d", m.Type)
	}
}

// MarshalSize returns the marshal size in bytes.
func (m Metadata) MarshalSize() (int, error) {
	payload, err := m.marshalPayload()
	if err != nil {
		return 0, err
	}

	return 1 + LEB128(uint32(len(payload))).MarshalSize() + len(payload), nil
}

// Marshal encodes the Metadata OBU.
// The returned OBU always contains the size field.
func (m Metadata) Marshal() ([]byte, error) {
	payload, err := m.marshalPayload()
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 1+LEB128(uint32(len(payload))).MarshalSize()+len(payload))
	buf[0] = byte(OBUTypeMetadata)<<3 | 0b00000010
	n := 1 + LEB128(uint32(len(payload))).MarshalTo(buf[1:])
	copy(buf[n:], payload)

	return buf, nil
}

// MetadataReader tracks the HDR metadata currently in effect while
// temporal units are processed in chronological order.
// When a temporal unit contains multiple Metadata OBUs of the same type,
// the last one wins and becomes the value returned by the next call.
type MetadataReader struct {
	cll  *Metadata_HDRContentLightLevel
	mdcv *Metadata_HDRMasteringDisplayColorVolume
}

// Read scans a temporal unit (a list of OBUs) in order and updates
// the values returned by ContentLightLevel and MasteringDisplayColorVolume.
// OBUs of other types are ignored, so their raw bytes and ordering are
// left untouched by the caller.
func (r *MetadataReader) Read(temporalUnit [][]byte) error {
	for _, obu := range temporalUnit {
		var oh OBUHeader
		err := oh.Unmarshal(obu)
		if err != nil {
			return err
		}

		if oh.Type != OBUTypeMetadata {
			continue
		}

		var m Metadata
		err = m.Unmarshal(obu)
		if err != nil {
			return err
		}

		switch m.Type {
		case MetadataTypeHDR_CLL:
			r.cll = m.HDRContentLightLevel

		case MetadataTypeHDR_MDCV:
			r.mdcv = m.HDRMasteringDisplayColorVolume
		}
	}

	return nil
}

// ContentLightLevel returns the last HDR_CLL value read so far, or nil.
func (r MetadataReader) ContentLightLevel() *Metadata_HDRContentLightLevel {
	return r.cll
}

// MasteringDisplayColorVolume returns the last HDR_MDCV value read so far, or nil.
func (r MetadataReader) MasteringDisplayColorVolume() *Metadata_HDRMasteringDisplayColorVolume {
	return r.mdcv
}
