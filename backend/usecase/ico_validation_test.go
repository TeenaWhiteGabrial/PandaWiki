package usecase

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"strings"
	"testing"
)

type icoTestFrame struct {
	width, height int
	data          []byte
}

func testICO(frames ...icoTestFrame) []byte {
	data := make([]byte, 6+16*len(frames))
	binary.LittleEndian.PutUint16(data[2:4], 1)
	binary.LittleEndian.PutUint16(data[4:6], uint16(len(frames)))
	for i, frame := range frames {
		entry := data[6+16*i : 6+16*(i+1)]
		entry[0], entry[1] = byte(frame.width), byte(frame.height)
		binary.LittleEndian.PutUint16(entry[4:6], 1)
		binary.LittleEndian.PutUint16(entry[6:8], 32)
		binary.LittleEndian.PutUint32(entry[8:12], uint32(len(frame.data)))
		binary.LittleEndian.PutUint32(entry[12:16], uint32(len(data)))
		data = append(data, frame.data...)
	}
	return data
}

func testICOPNG(width, height int) []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, width, height)))
	return b.Bytes()
}

func testICOBitmap(bits, width, height int) []byte {
	colors := 0
	if bits <= 8 {
		colors = 1 << bits
	}
	stride := (width*bits + 31) / 32 * 4
	maskSize := (width + 31) / 32 * 4 * height
	data := make([]byte, 40+colors*4+stride*height+maskSize)
	binary.LittleEndian.PutUint32(data[:4], 40)
	binary.LittleEndian.PutUint32(data[4:8], uint32(width))
	binary.LittleEndian.PutUint32(data[8:12], uint32(2*height))
	binary.LittleEndian.PutUint16(data[12:14], 1)
	binary.LittleEndian.PutUint16(data[14:16], uint16(bits))
	return data
}

func TestICOImageUploadCompatibility(t *testing.T) {
	pngFrame := icoTestFrame{16, 16, testICOPNG(16, 16)}
	cases := map[string][]byte{
		"PNG":         testICO(pngFrame),
		"256px PNG":   testICO(icoTestFrame{256, 256, testICOPNG(256, 256)}),
		"multi-image": testICO(pngFrame, icoTestFrame{32, 32, testICOBitmap(32, 32, 32)}),
	}
	for _, bits := range []int{1, 4, 8, 16, 24, 32} {
		cases[fmt.Sprintf("%d-bit bitmap", bits)] = testICO(icoTestFrame{17, 13, testICOBitmap(bits, 17, 13)})
	}
	noMask := testICOBitmap(32, 16, 16)
	cases["32-bit alpha without AND mask"] = testICO(icoTestFrame{16, 16, noMask[:len(noMask)-64]})
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			for _, imageOnly := range []bool{true, false} {
				src := bytes.NewReader(data)
				contentType, err := validateUpload("favicon.ICO", src, int64(len(data)), imageOnly)
				if err != nil || contentType != "image/vnd.microsoft.icon" {
					t.Fatalf("valid ICO rejected: MIME=%q err=%v", contentType, err)
				}
				if pos, _ := src.Seek(0, 1); pos != 0 {
					t.Fatal("upload source not rewound")
				}
			}
		})
	}
}

func TestICOImageUploadRejectsMalformedAndSpoofedContent(t *testing.T) {
	frame := icoTestFrame{16, 16, testICOPNG(16, 16)}
	valid := testICO(frame)
	mutate := func(offset int, data []byte) []byte {
		result := append([]byte(nil), valid...)
		copy(result[offset:], data)
		return result
	}
	bitmap := func(change func([]byte)) []byte {
		data := testICOBitmap(24, 16, 16)
		change(data)
		return testICO(icoTestFrame{16, 16, data})
	}
	shortPalette := testICOBitmap(8, 16, 16)
	shortPalette = append(append([]byte(nil), shortPalette[:44]...), shortPalette[40+256*4:]...)
	binary.LittleEndian.PutUint32(shortPalette[32:36], 1)
	shortPalette[44] = 1
	cases := map[string][]byte{
		"HTML renamed ICO":       []byte("<html><script>alert(1)</script></html>"),
		"SVG renamed ICO":        []byte("<svg onload='alert(1)'/>"),
		"PNG renamed ICO":        frame.data,
		"empty directory":        {0, 0, 1, 0, 0, 0},
		"cursor instead of icon": mutate(2, []byte{2, 0}),
		"oversized directory":    mutate(4, []byte{1, 1}),
		"truncated directory":    valid[:20],
		"directory overlap":      mutate(18, []byte{0, 0, 0, 0}),
		"offset overflow":        mutate(18, []byte{255, 255, 255, 255}),
		"length overflow":        mutate(14, []byte{255, 255, 255, 255}),
		"wrong PNG dimensions":   mutate(6, []byte{32}),
		"truncated PNG":          testICO(icoTestFrame{16, 16, frame.data[:len(frame.data)/2]}),
		"invalid second frame":   testICO(frame, icoTestFrame{16, 16, []byte("not an image")}),
		"wrong bitmap dimensions": bitmap(func(b []byte) {
			binary.LittleEndian.PutUint32(b[4:8], 0x7fffffff)
		}),
		"wrong bitmap height": bitmap(func(b []byte) {
			binary.LittleEndian.PutUint32(b[8:12], 16)
		}),
		"invalid bitmap compression": bitmap(func(b []byte) {
			binary.LittleEndian.PutUint32(b[16:20], 1)
		}),
		"palette index overflow": testICO(icoTestFrame{16, 16, shortPalette}),
		"missing bitmap mask":    testICO(icoTestFrame{16, 16, testICOBitmap(24, 16, 16)[:40+16*3*16]}),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := validateUpload("favicon.ico", bytes.NewReader(data), int64(len(data)), true); err == nil {
				t.Fatal("invalid ICO accepted")
			}
		})
	}
	if _, err := validateUpload("spoof.png", bytes.NewReader(valid), int64(len(valid)), true); err == nil || !strings.Contains(err.Error(), "文件实际为 ICO 图标") {
		t.Fatalf("ICO renamed PNG must have a precise format mismatch error, got %v", err)
	}
}

func FuzzICOValidation(f *testing.F) {
	f.Add(testICO(icoTestFrame{16, 16, testICOPNG(16, 16)}))
	f.Add(testICO(icoTestFrame{16, 16, testICOBitmap(32, 16, 16)}))
	f.Add([]byte{0, 0, 1, 0, 0, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 20*1024*1024 {
			t.Skip()
		}
		// Exercise the parser directly: malformed input must return an error,
		// not rely on validateUpload's outer panic recovery.
		_ = validateICO(bytes.NewReader(data), int64(len(data)))
	})
}
