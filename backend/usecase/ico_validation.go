package usecase

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image/png"
	"io"

	"golang.org/x/image/bmp"
)

// ICO frames contain either PNG data or a DIB followed by a transparency mask.
// Validate every directory entry, not only the image selected for display.
// https://devblogs.microsoft.com/oldnewthing/20101019-00/?p=12503
func validateICO(src io.ReaderAt, size int64) error {
	var header [6]byte
	if _, err := src.ReadAt(header[:], 0); err != nil ||
		binary.LittleEndian.Uint16(header[:2]) != 0 || binary.LittleEndian.Uint16(header[2:4]) != 1 {
		return fmt.Errorf("ICO 图标文件头无效")
	}
	count := int64(binary.LittleEndian.Uint16(header[4:6]))
	if count == 0 || count > 256 || 6+16*count > size {
		return fmt.Errorf("ICO 图标目录无效或图像数量超过 256")
	}
	var totalFrameBytes int64
	for i := int64(0); i < count; i++ {
		var entry [16]byte
		if _, err := src.ReadAt(entry[:], 6+16*i); err != nil || entry[3] != 0 {
			return fmt.Errorf("ICO 图标目录损坏")
		}
		width, height := int(entry[0]), int(entry[1])
		if width == 0 {
			width = 256
		}
		if height == 0 {
			height = 256
		}
		length := int64(binary.LittleEndian.Uint32(entry[8:12]))
		offset := int64(binary.LittleEndian.Uint32(entry[12:16]))
		if length < 8 || offset < 6+16*count || offset > size || length > size-offset {
			return fmt.Errorf("ICO 图标图像数据越界或不完整")
		}
		totalFrameBytes += length
		if totalFrameBytes > 20*1024*1024 {
			return fmt.Errorf("ICO 图标累计图像数据不能超过 20 MB")
		}
		frame := io.NewSectionReader(src, offset, length)
		var signature [8]byte
		if _, err := frame.ReadAt(signature[:], 0); err != nil {
			return fmt.Errorf("ICO 图标图像数据不完整")
		}
		if string(signature[:]) == "\x89PNG\r\n\x1a\n" {
			cfg, err := png.DecodeConfig(frame)
			if err != nil || cfg.Width != width || cfg.Height != height {
				return fmt.Errorf("ICO 图标 PNG 内容无效或尺寸与目录不一致")
			}
			_, _ = frame.Seek(0, io.SeekStart)
			if _, err := png.Decode(frame); err != nil {
				return fmt.Errorf("ICO 图标 PNG 解码失败：%w", err)
			}
		} else {
			data, err := io.ReadAll(frame)
			if err != nil {
				return fmt.Errorf("ICO 图标位图读取失败：%w", err)
			}
			if err := validateICOBitmap(data, width, height); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateICOBitmap(data []byte, width, height int) error {
	if len(data) < 40 {
		return fmt.Errorf("ICO 图标位图头不完整")
	}
	headerSize := int(binary.LittleEndian.Uint32(data[:4]))
	if (headerSize != 40 && headerSize != 108 && headerSize != 124) || headerSize > len(data) {
		return fmt.Errorf("不支持的 ICO 位图头，请转换为 PNG 图标后上传")
	}
	if int32(binary.LittleEndian.Uint32(data[4:8])) != int32(width) ||
		int32(binary.LittleEndian.Uint32(data[8:12])) != int32(2*height) ||
		binary.LittleEndian.Uint16(data[12:14]) != 1 {
		return fmt.Errorf("ICO 位图尺寸或颜色平面无效")
	}
	bits := int(binary.LittleEndian.Uint16(data[14:16]))
	if bits != 1 && bits != 4 && bits != 8 && bits != 16 && bits != 24 && bits != 32 {
		return fmt.Errorf("ICO 位图颜色深度无效")
	}
	compression := binary.LittleEndian.Uint32(data[16:20])
	if compression != 0 && !(compression == 3 && headerSize >= 108 && (bits == 24 || bits == 32)) {
		return fmt.Errorf("不支持的 ICO 位图压缩方式，请转换为 PNG 图标后上传")
	}
	colors := 0
	if bits <= 8 {
		colorCount := binary.LittleEndian.Uint32(data[32:36])
		if colorCount > 1<<bits {
			return fmt.Errorf("ICO 位图调色板无效")
		}
		colors = int(colorCount)
		if colors == 0 {
			colors = 1 << bits
		}
	}
	pixelOffset := headerSize + colors*4
	stride := ((width*bits + 31) / 32) * 4
	pixelEnd := pixelOffset + stride*height
	maskSize := ((width + 31) / 32) * 4 * height
	if pixelEnd > len(data) || (len(data)-pixelEnd < maskSize && !(bits == 32 && len(data) == pixelEnd)) {
		return fmt.Errorf("ICO 位图像素或透明掩码不完整")
	}

	// Expand packed palettes and 16-bit RGB into a format supported by the
	// existing Go BMP decoder. This also checks every palette index.
	if bits <= 8 || bits == 16 {
		outStride := (width*3 + 3) &^ 3
		expanded := make([]byte, 40+outStride*height)
		binary.LittleEndian.PutUint32(expanded[:4], 40)
		binary.LittleEndian.PutUint32(expanded[4:8], uint32(width))
		binary.LittleEndian.PutUint32(expanded[8:12], uint32(height))
		binary.LittleEndian.PutUint16(expanded[12:14], 1)
		binary.LittleEndian.PutUint16(expanded[14:16], 24)
		for y := 0; y < height; y++ {
			row := data[pixelOffset+y*stride : pixelOffset+(y+1)*stride]
			for x := 0; x < width; x++ {
				out := expanded[40+y*outStride+x*3:][:3]
				if bits == 16 {
					pixel := binary.LittleEndian.Uint16(row[x*2:])
					out[0], out[1], out[2] = byte((pixel&31)*255/31), byte(((pixel>>5)&31)*255/31), byte(((pixel>>10)&31)*255/31)
				} else {
					index := int(row[x*bits/8]>>uint(8-bits-(x*bits)%8)) & ((1 << bits) - 1)
					if index >= colors {
						return fmt.Errorf("ICO 位图调色板索引越界")
					}
					copy(out, data[headerSize+index*4:][:3])
				}
			}
		}
		data, pixelOffset = expanded, 40
	} else {
		data = append([]byte(nil), data[:pixelEnd]...)
		binary.LittleEndian.PutUint32(data[8:12], uint32(height))
	}
	var fileHeader [14]byte
	copy(fileHeader[:2], "BM")
	binary.LittleEndian.PutUint32(fileHeader[2:6], uint32(14+len(data)))
	binary.LittleEndian.PutUint32(fileHeader[10:14], uint32(14+pixelOffset))
	if _, err := bmp.Decode(io.MultiReader(bytes.NewReader(fileHeader[:]), bytes.NewReader(data))); err != nil {
		return fmt.Errorf("ICO 图标位图解码失败：%w", err)
	}
	return nil
}
