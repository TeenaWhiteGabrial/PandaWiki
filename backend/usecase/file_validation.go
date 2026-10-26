package usecase

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

const maxUploadBytes int64 = 1024 * 1024 * 1024

type uploadSource interface {
	io.ReadSeeker
	io.ReaderAt
}

var uploadTypes = map[string]string{
	".pdf": "application/pdf", ".txt": "text/plain; charset=utf-8", ".md": "text/plain; charset=utf-8", ".csv": "text/plain; charset=utf-8", ".json": "text/plain; charset=utf-8", ".xml": "text/plain; charset=utf-8", ".log": "text/plain; charset=utf-8",
	".doc": "application/msword", ".xls": "application/vnd.ms-excel", ".ppt": "application/vnd.ms-powerpoint",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document", ".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".zip": "application/zip", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".gif": "image/gif", ".webp": "image/webp", ".bmp": "image/bmp", ".tif": "image/tiff", ".tiff": "image/tiff",
	".ico": "image/vnd.microsoft.icon",
	".mp4": "video/mp4", ".m4v": "video/mp4", ".mov": "video/quicktime", ".avi": "video/x-msvideo", ".webm": "video/webm", ".mkv": "video/x-matroska", ".mp3": "audio/mpeg", ".wav": "audio/wav", ".ogg": "audio/ogg", ".flac": "audio/flac", ".m4a": "audio/mp4",
}

func validateUpload(filename string, src uploadSource, size int64, imageOnly bool) (contentType string, err error) {
	// The parser is defensive, but a malformed PDF must never panic the API.
	defer func() {
		if recover() != nil {
			contentType = ""
			err = fmt.Errorf("文件内容损坏或无法安全解析")
		}
		_, _ = src.Seek(0, io.SeekStart)
	}()
	ext := strings.ToLower(filepath.Ext(filename))
	contentType, ok := uploadTypes[ext]
	if !ok {
		return "", fmt.Errorf("不支持上传此文件类型：%s", ext)
	}
	if size <= 0 || size > maxUploadBytes {
		return "", fmt.Errorf("文件大小须在 1 字节至 1 GB 之间")
	}
	if imageOnly && !strings.HasPrefix(contentType, "image/") {
		return "", fmt.Errorf("只支持经过解码校验的图片")
	}
	head := make([]byte, 512)
	n, readErr := src.ReadAt(head, 0)
	if readErr != nil && readErr != io.EOF {
		return "", readErr
	}
	head = head[:n]
	switch {
	case strings.HasPrefix(contentType, "image/"):
		if size > 20*1024*1024 {
			return "", fmt.Errorf("图片不能超过 20 MB")
		}
		if ext == ".ico" {
			if err = validateICO(src, size); err != nil {
				return "", err
			}
			break
		}
		if bytes.HasPrefix(head, []byte{0, 0, 1, 0}) {
			return "", fmt.Errorf("文件实际为 ICO 图标，与扩展名 %s 不一致；请转换为 PNG 或使用 .ico 文件上传", ext)
		}
		if _, err = src.Seek(0, io.SeekStart); err != nil {
			return "", err
		}
		cfg, format, e := image.DecodeConfig(src)
		if e != nil {
			return "", fmt.Errorf("图片格式无法识别或文件损坏（扩展名 %s，检测类型 %s）：%w", ext, http.DetectContentType(head), e)
		}
		if cfg.Width <= 0 || cfg.Height <= 0 {
			return "", fmt.Errorf("图片尺寸无效（%d × %d）", cfg.Width, cfg.Height)
		}
		if int64(cfg.Width) > 40000000/int64(cfg.Height) {
			return "", fmt.Errorf("图片像素数超过 4000 万（%d × %d）", cfg.Width, cfg.Height)
		}
		if !(ext == "."+format || (format == "jpeg" && ext == ".jpg") || (format == "tiff" && ext == ".tif")) {
			return "", fmt.Errorf("图片真实格式与扩展名不一致")
		}
		_, _ = src.Seek(0, io.SeekStart)
		if _, _, err = image.Decode(src); err != nil {
			return "", fmt.Errorf("图片解码失败：%w", err)
		}
	case ext == ".pdf":
		if size > 100*1024*1024 {
			return "", fmt.Errorf("PDF 安全检查支持最大 100 MB")
		}
		if err = validatePDF(src, size); err != nil {
			return "", err
		}
	case ext == ".zip" || ext == ".docx" || ext == ".xlsx" || ext == ".pptx":
		z, e := zip.NewReader(src, size)
		if e != nil {
			return "", fmt.Errorf("ZIP/Office 文件结构无效")
		}
		if len(z.File) > 10000 {
			return "", fmt.Errorf("压缩包文件数过多")
		}
		var total uint64
		officeRoot := map[string]string{".docx": "word/", ".xlsx": "xl/", ".pptx": "ppt/"}[ext]
		hasTypes, hasRoot := false, false
		for _, f := range z.File {
			total += f.UncompressedSize64
			if total > 2*1024*1024*1024 {
				return "", fmt.Errorf("压缩包解压大小超过 2 GB")
			}
			name := strings.ToLower(f.Name)
			hasTypes = hasTypes || name == "[content_types].xml"
			hasRoot = hasRoot || strings.HasPrefix(name, officeRoot)
			if officeRoot != "" && (strings.Contains(name, "vbaproject") || strings.Contains(name, "/activex/") || strings.Contains(name, "/embeddings/")) {
				return "", fmt.Errorf("不支持带宏、ActiveX 或嵌入程序的 Office 文件")
			}
		}
		if officeRoot != "" && (!hasTypes || !hasRoot) {
			return "", fmt.Errorf("Office 文件真实格式与扩展名不一致")
		}
	case ext == ".doc" || ext == ".xls" || ext == ".ppt":
		if !bytes.HasPrefix(head, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}) {
			return "", fmt.Errorf("Office 文件头无效")
		}
	case strings.HasPrefix(contentType, "text/plain"):
		// Text is deliberately stored as text/plain, never text/html/SVG/XML.
		if bytes.IndexByte(head, 0) >= 0 {
			return "", fmt.Errorf("文本文件包含二进制内容")
		}
		// A trailing partial UTF-8 rune in the sniff buffer is valid.
		end := len(head)
		for end > 0 && !utf8.Valid(head[:end]) && end > len(head)-4 {
			end--
		}
		if !utf8.Valid(head[:end]) {
			return "", fmt.Errorf("文本文件须使用 UTF-8 编码")
		}
	default:
		match := false
		switch ext {
		case ".mp4", ".m4v", ".mov", ".m4a":
			match = len(head) >= 12 && string(head[4:8]) == "ftyp"
		case ".avi":
			match = len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "AVI "
		case ".wav":
			match = len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WAVE"
		case ".webm", ".mkv":
			match = bytes.HasPrefix(head, []byte{0x1a, 0x45, 0xdf, 0xa3})
		case ".mp3":
			match = bytes.HasPrefix(head, []byte("ID3")) || (len(head) > 1 && head[0] == 0xff && head[1]&0xe0 == 0xe0)
		case ".ogg":
			match = bytes.HasPrefix(head, []byte("OggS"))
		case ".flac":
			match = bytes.HasPrefix(head, []byte("fLaC"))
		}
		if !match {
			return "", fmt.Errorf("文件真实类型与扩展名不一致（检测为 %s）", http.DetectContentType(head))
		}
	}
	return contentType, nil
}

// Inspect the parsed object graph instead of searching raw bytes: PDF names may
// be escaped and objects may reside inside compressed object streams.
func validatePDF(src io.ReaderAt, size int64) error {
	reader, err := pdf.NewReader(src, size)
	if err != nil {
		return fmt.Errorf("PDF 损坏、加密或无法安全解析：%w", err)
	}
	if reader.Trailer().Key("Root").IsNull() {
		return fmt.Errorf("PDF 缺少文档根对象")
	}
	seen := make(map[[32]byte]bool)
	count := 0
	var walk func(pdf.Value, int) error
	walk = func(v pdf.Value, depth int) error {
		kind := v.Kind()
		if kind != pdf.Dict && kind != pdf.Array && kind != pdf.Stream {
			return nil
		}
		if depth > 128 || count > 100000 {
			return fmt.Errorf("PDF 对象结构过于复杂，无法安全检查")
		}
		key := sha256.Sum256([]byte(v.String()))
		if seen[key] {
			return nil
		}
		seen[key] = true
		count++
		if kind == pdf.Array {
			for i := 0; i < v.Len(); i++ {
				if err := walk(v.Index(i), depth+1); err != nil {
					return err
				}
			}
			return nil
		}
		for _, name := range v.Keys() {
			switch name {
			case "JS", "JavaScript", "AA", "Launch", "RichMedia", "XFA", "EmbeddedFiles":
				return fmt.Errorf("PDF 包含不允许的主动内容：%s", name)
			}
			if name == "OpenAction" {
				action := v.Key(name)
				// Ordinary PDFs use an opening destination or local GoTo for
				// initial page/zoom. Permit those; reject executable actions.
				if action.Kind() != pdf.Array && action.Key("S").Name() != "GoTo" {
					return fmt.Errorf("PDF 包含不安全的打开动作")
				}
			}
			if name == "S" {
				switch v.Key(name).Name() {
				case "JavaScript", "Launch", "SubmitForm", "ImportData", "GoToR", "GoToE", "Rendition", "Movie", "Sound":
					return fmt.Errorf("PDF 包含不允许的动作")
				}
			}
			if name == "URI" {
				target, e := url.Parse(v.Key(name).RawString())
				if e != nil || (target.Scheme != "https" && target.Scheme != "http" && target.Scheme != "mailto") {
					return fmt.Errorf("PDF 包含不安全的外部链接")
				}
			}
			if e := walk(v.Key(name), depth+1); e != nil {
				return e
			}
		}
		return nil
	}
	return walk(reader.Trailer(), 0)
}

func stageUpload(reader io.Reader, size int64) (*os.File, error) {
	if size <= 0 || size > maxUploadBytes {
		return nil, fmt.Errorf("文件大小须在 1 字节至 1 GB 之间")
	}
	f, err := os.CreateTemp("", "pandawiki-upload-*")
	if err != nil {
		return nil, err
	}
	n, err := io.Copy(f, io.LimitReader(reader, size+1))
	if err != nil || n != size {
		f.Close()
		os.Remove(f.Name())
		return nil, fmt.Errorf("读取上传文件失败或文件大小不匹配")
	}
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, err
	}
	return f, nil
}
