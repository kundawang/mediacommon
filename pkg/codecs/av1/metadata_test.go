package av1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var hdrCLL1 = []byte{0x2A, 0x05, 0x01, 0x03, 0xE8, 0x01, 0x90}

var hdrCLL2 = []byte{0x2A, 0x05, 0x01, 0x0F, 0xA0, 0x03, 0xE8}

var hdrCLL3 = []byte{0x2A, 0x05, 0x01, 0x07, 0xD0, 0x02, 0x58}

var hdrMDCV1 = []byte{
	0x2A, 0x19, 0x02,
	0x84, 0xD0, 0x3E, 0x80,
	0x33, 0xC2, 0x86, 0xC4,
	0x1D, 0x4C, 0x0B, 0xB8,
	0x3D, 0x13, 0x40, 0x42,
	0x00, 0x98, 0x96, 0x80,
	0x00, 0x00, 0x00, 0x32,
}

var hdrMDCV2 = []byte{
	0x2A, 0x19, 0x02,
	0x8A, 0x48, 0x39, 0x08,
	0x21, 0x34, 0x9B, 0xAA,
	0x19, 0x96, 0x08, 0xFC,
	0x3D, 0x13, 0x40, 0x42,
	0x00, 0x98, 0x96, 0x80,
	0x00, 0x00, 0x00, 0x01,
}

var decCLL1 = Metadata{
	Type: MetadataTypeHDR_CLL,
	HDRContentLightLevel: &Metadata_HDRContentLightLevel{
		MaxContentLightLevel:      1000,
		MaxFrameAverageLightLevel: 400,
	},
}

var decCLL3 = Metadata{
	Type: MetadataTypeHDR_CLL,
	HDRContentLightLevel: &Metadata_HDRContentLightLevel{
		MaxContentLightLevel:      2000,
		MaxFrameAverageLightLevel: 600,
	},
}

var decMDCV1 = Metadata{
	Type: MetadataTypeHDR_MDCV,
	HDRMasteringDisplayColorVolume: &Metadata_HDRMasteringDisplayColorVolume{
		PrimaryRChromaticityX:   34000,
		PrimaryRChromaticityY:   16000,
		PrimaryGChromaticityX:   13250,
		PrimaryGChromaticityY:   34500,
		PrimaryBChromaticityX:   7500,
		PrimaryBChromaticityY:   3000,
		WhitePointChromaticityX: 15635,
		WhitePointChromaticityY: 16450,
		MaxMasteringLuminance:   10000000,
		MinMasteringLuminance:   50,
	},
}

var decMDCV2 = Metadata{
	Type: MetadataTypeHDR_MDCV,
	HDRMasteringDisplayColorVolume: &Metadata_HDRMasteringDisplayColorVolume{
		PrimaryRChromaticityX:   35400,
		PrimaryRChromaticityY:   14600,
		PrimaryGChromaticityX:   8500,
		PrimaryGChromaticityY:   39850,
		PrimaryBChromaticityX:   6550,
		PrimaryBChromaticityY:   2300,
		WhitePointChromaticityX: 15635,
		WhitePointChromaticityY: 16450,
		MaxMasteringLuminance:   10000000,
		MinMasteringLuminance:   1,
	},
}

func TestMetadataUnmarshal(t *testing.T) {
	for _, ca := range []struct {
		name string
		byts []byte
		dec  Metadata
	}{
		{"hdr cll", hdrCLL1, decCLL1},
		{"hdr mdcv", hdrMDCV1, decMDCV1},
		{
			"hdr cll without size field",
			append([]byte{0x28}, hdrCLL1[2:]...),
			decCLL1,
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var dec Metadata
			err := dec.Unmarshal(ca.byts)
			require.NoError(t, err)
			require.Equal(t, ca.dec, dec)
		})
	}
}

func TestMetadataMarshal(t *testing.T) {
	for _, ca := range []struct {
		name string
		byts []byte
		dec  Metadata
	}{
		{"hdr cll", hdrCLL1, decCLL1},
		{"hdr mdcv", hdrMDCV1, decMDCV1},
	} {
		t.Run(ca.name, func(t *testing.T) {
			enc, err := ca.dec.Marshal()
			require.NoError(t, err)
			require.Equal(t, ca.byts, enc)

			size, err := ca.dec.MarshalSize()
			require.NoError(t, err)
			require.Equal(t, len(enc), size)
		})
	}
}

func TestMetadataUnmarshalErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		byts []byte
	}{
		{"empty", []byte{}},
		{"wrong obu type", []byte{0x12, 0x00}},
		{"truncated obu size leb128", []byte{0x2A, 0x80}},
		{"obu size exceeds 32 bits", []byte{0x2A, 0xFF, 0xFF, 0xFF, 0xFF, 0x1F}},
		{"declared size larger than payload", []byte{0x2A, 0x05, 0x01, 0x03, 0xE8}},
		{"truncated metadata type leb128", []byte{0x2A, 0x01, 0x80}},
		{"metadata type exceeds 32 bits", []byte{0x2A, 0x05, 0xFF, 0xFF, 0xFF, 0xFF, 0x1F}},
		{"cll payload too short", []byte{0x2A, 0x04, 0x01, 0x03, 0xE8, 0x01}},
		{"cll payload too long", []byte{0x2A, 0x06, 0x01, 0x03, 0xE8, 0x01, 0x90, 0x00}},
		{
			"mdcv payload truncated",
			[]byte{
				0x2A, 0x18, 0x02,
				0x84, 0xD0, 0x3E, 0x80,
				0x33, 0xC2, 0x86, 0xC4,
				0x1D, 0x4C, 0x0B, 0xB8,
				0x3D, 0x13, 0x40, 0x42,
				0x00, 0x98, 0x96, 0x80,
				0x00, 0x00,
			},
		},
		{"mdcv payload too long", func() []byte {
			b := append(append([]byte{}, hdrMDCV1...), 0x00)
			b[1] = 0x1A
			return b
		}()},
		{"unknown metadata type", []byte{0x2A, 0x01, 0x63}},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var dec Metadata
			err := dec.Unmarshal(ca.byts)
			require.Error(t, err)
		})
	}
}

func TestMetadataMarshalErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		dec  Metadata
	}{
		{
			"cll without payload",
			Metadata{Type: MetadataTypeHDR_CLL},
		},
		{
			"cll with mdcv payload",
			Metadata{
				Type:                           MetadataTypeHDR_CLL,
				HDRMasteringDisplayColorVolume: &Metadata_HDRMasteringDisplayColorVolume{},
			},
		},
		{
			"mdcv without payload",
			Metadata{Type: MetadataTypeHDR_MDCV},
		},
		{
			"mdcv with cll payload",
			Metadata{
				Type:                 MetadataTypeHDR_MDCV,
				HDRContentLightLevel: &Metadata_HDRContentLightLevel{},
			},
		},
		{
			"unknown metadata type",
			Metadata{Type: MetadataType(99)},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			_, err := ca.dec.Marshal()
			require.Error(t, err)

			_, err = ca.dec.MarshalSize()
			require.Error(t, err)
		})
	}
}

func TestMetadataReader(t *testing.T) {
	tu1 := [][]byte{
		{0x12, 0x00},
		{0x7A, 0x02, 0xFF, 0xFF},
		hdrCLL1,
		hdrMDCV1,
	}
	tu2 := [][]byte{
		{0x12, 0x00},
		hdrCLL2,
		hdrCLL3,
	}
	tu3 := [][]byte{
		{0x12, 0x00},
		hdrMDCV2,
		{0x7A, 0x02, 0xAA, 0xBB},
	}

	var r MetadataReader
	require.Nil(t, r.ContentLightLevel())
	require.Nil(t, r.MasteringDisplayColorVolume())

	require.NoError(t, r.Read(tu1))
	require.Equal(t, decCLL1.HDRContentLightLevel, r.ContentLightLevel())
	require.Equal(t, decMDCV1.HDRMasteringDisplayColorVolume, r.MasteringDisplayColorVolume())

	require.NoError(t, r.Read(tu2))
	require.Equal(t, decCLL3.HDRContentLightLevel, r.ContentLightLevel())
	require.Equal(t, decMDCV1.HDRMasteringDisplayColorVolume, r.MasteringDisplayColorVolume())

	require.NoError(t, r.Read(tu3))
	require.Equal(t, decCLL3.HDRContentLightLevel, r.ContentLightLevel())
	require.Equal(t, decMDCV2.HDRMasteringDisplayColorVolume, r.MasteringDisplayColorVolume())

	// the reader must not modify the temporal units
	require.Equal(t, [][]byte{
		{0x12, 0x00},
		{0x7A, 0x02, 0xFF, 0xFF},
		hdrCLL1,
		hdrMDCV1,
	}, tu1)
}

func TestMetadataReaderError(t *testing.T) {
	var r MetadataReader
	err := r.Read([][]byte{
		{0x12, 0x00},
		{0x2A, 0x01, 0x63},
	})
	require.Error(t, err)
}

func TestMetadataPreservesOtherOBUs(t *testing.T) {
	tu := [][]byte{
		{0x12, 0x00},
		hdrMDCV1,
		{0x7A, 0x02, 0xFF, 0xFF},
		hdrCLL1,
		{0x7A, 0x01, 0x00},
	}

	out := make([][]byte, len(tu))
	for i, obu := range tu {
		var oh OBUHeader
		err := oh.Unmarshal(obu)
		require.NoError(t, err)

		if oh.Type == OBUTypeMetadata {
			var m Metadata
			err = m.Unmarshal(obu)
			require.NoError(t, err)

			out[i], err = m.Marshal()
			require.NoError(t, err)
		} else {
			out[i] = obu
		}
	}

	// metadata OBUs are regenerated identically, other OBUs keep raw bytes and order
	require.Equal(t, tu, out)
}

func TestMetadataWithBitstreamRoundTrip(t *testing.T) {
	tu1 := [][]byte{
		{0x12, 0x00},
		hdrCLL1,
		hdrMDCV1,
	}
	tu2 := [][]byte{
		{0x12, 0x00},
		hdrCLL3,
		hdrMDCV2,
	}

	enc1, err := Bitstream(tu1).Marshal()
	require.NoError(t, err)
	enc2, err := Bitstream(tu2).Marshal()
	require.NoError(t, err)

	var bs1 Bitstream
	require.NoError(t, bs1.Unmarshal(enc1))
	var bs2 Bitstream
	require.NoError(t, bs2.Unmarshal(enc2))

	var r MetadataReader
	require.NoError(t, r.Read(bs1))
	require.Equal(t, decCLL1.HDRContentLightLevel, r.ContentLightLevel())
	require.Equal(t, decMDCV1.HDRMasteringDisplayColorVolume, r.MasteringDisplayColorVolume())

	require.NoError(t, r.Read(bs2))
	require.Equal(t, decCLL3.HDRContentLightLevel, r.ContentLightLevel())
	require.Equal(t, decMDCV2.HDRMasteringDisplayColorVolume, r.MasteringDisplayColorVolume())

	// the original bitstream bytes are preserved through the round trip
	bs1Enc, err := bs1.Marshal()
	require.NoError(t, err)
	require.Equal(t, enc1, bs1Enc)
	bs2Enc, err := bs2.Marshal()
	require.NoError(t, err)
	require.Equal(t, enc2, bs2Enc)
}

func FuzzMetadataUnmarshal(f *testing.F) {
	for _, b := range [][]byte{hdrCLL1, hdrMDCV1, hdrCLL2, hdrMDCV2} {
		f.Add(b)
	}

	f.Fuzz(func(t *testing.T, b []byte) {
		var m Metadata
		err := m.Unmarshal(b)
		if err != nil {
			return
		}

		enc, err := m.Marshal()
		require.NoError(t, err)

		var m2 Metadata
		err = m2.Unmarshal(enc)
		require.NoError(t, err)
		require.Equal(t, m, m2)
	})
}
