// Package apkinfo membaca identitas paket Android (package, versionCode,
// versionName) langsung dari AndroidManifest.xml biner di dalam APK, tanpa
// aapt. Dipakai halaman App POS & pembaruan dalam-aplikasi untuk mengetahui
// versi tiap APK yang diunggah.
package apkinfo

import (
	"archive/zip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode/utf16"
)

// Info identitas satu APK.
type Info struct {
	Package     string
	VersionCode int64
	VersionName string
}

var errFormat = errors.New("AndroidManifest.xml biner tidak valid")

// Batas aman ukuran manifest yang dibaca ke memori (manifest normal < 100 KB).
const maxManifestBytes = 4 << 20

// ReadFile membaca identitas paket dari berkas APK di path.
func ReadFile(path string) (Info, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return Info{}, fmt.Errorf("bukan arsip APK: %w", err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != "AndroidManifest.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return Info{}, err
		}
		defer rc.Close()
		data, err := io.ReadAll(io.LimitReader(rc, maxManifestBytes+1))
		if err != nil {
			return Info{}, err
		}
		if len(data) > maxManifestBytes {
			return Info{}, errors.New("AndroidManifest.xml terlalu besar")
		}
		return ParseManifest(data)
	}
	return Info{}, errors.New("AndroidManifest.xml tidak ditemukan di APK")
}

// Konstanta format XML biner Android (frameworks/base ResourceTypes.h).
const (
	chunkXML          = 0x0003
	chunkStringPool   = 0x0001
	chunkResourceMap  = 0x0180
	chunkStartElement = 0x0102

	stringPoolUTF8 = 1 << 8

	typeString = 0x03
	typeIntDec = 0x10
	typeIntHex = 0x11

	// android:versionCode / android:versionName (android.R.attr).
	attrVersionCode = 0x0101021b
	attrVersionName = 0x0101021c

	noIndex = 0xffffffff
)

// ParseManifest membaca atribut elemen <manifest> dari XML biner.
func ParseManifest(b []byte) (Info, error) {
	if len(b) < 8 || le16(b, 0) != chunkXML {
		return Info{}, errFormat
	}
	total := int(le32(b, 4))
	if total > len(b) || total < 8 {
		total = len(b)
	}
	var pool []string
	var resMap []uint32
	off := int(le16(b, 2))
	for off+8 <= total {
		typ := le16(b, off)
		headerSize := int(le16(b, off+2))
		size := int(le32(b, off+4))
		if size < 8 || headerSize < 8 || headerSize > size || off+size > total {
			return Info{}, errFormat
		}
		chunk := b[off : off+size]
		switch typ {
		case chunkStringPool:
			var err error
			if pool, err = parseStringPool(chunk); err != nil {
				return Info{}, err
			}
		case chunkResourceMap:
			for i := headerSize; i+4 <= size; i += 4 {
				resMap = append(resMap, le32(chunk, i))
			}
		case chunkStartElement:
			info, isManifest, err := parseStartElement(chunk, headerSize, pool, resMap)
			if err != nil {
				return Info{}, err
			}
			if isManifest {
				return info, nil
			}
		}
		off += size
	}
	return Info{}, errors.New("elemen <manifest> tidak ditemukan")
}

func parseStartElement(c []byte, headerSize int, pool []string, resMap []uint32) (Info, bool, error) {
	// ResXMLTree_attrExt setelah header: ns, name, attributeStart,
	// attributeSize, attributeCount, idIndex, classIndex, styleIndex.
	ext := headerSize
	if ext+20 > len(c) {
		return Info{}, false, errFormat
	}
	if str(pool, le32(c, ext+4)) != "manifest" {
		return Info{}, false, nil
	}
	attrStart := int(le16(c, ext+8))
	attrSize := int(le16(c, ext+10))
	count := int(le16(c, ext+12))
	if attrSize < 20 {
		return Info{}, false, errFormat
	}
	var info Info
	for i := 0; i < count; i++ {
		a := ext + attrStart + i*attrSize
		if a+20 > len(c) {
			return Info{}, false, errFormat
		}
		nameIdx := le32(c, a+4)
		raw := le32(c, a+8)
		dataType := c[a+15]
		data := le32(c, a+16)

		name := str(pool, nameIdx)
		var resID uint32
		if int(nameIdx) < len(resMap) {
			resID = resMap[nameIdx]
		}
		switch {
		case name == "package" && resID == 0:
			info.Package = value(pool, raw, dataType, data)
		case resID == attrVersionCode || (resID == 0 && name == "versionCode"):
			if dataType == typeIntDec || dataType == typeIntHex {
				info.VersionCode = int64(data)
			} else if v, err := strconv.ParseInt(value(pool, raw, dataType, data), 10, 64); err == nil {
				info.VersionCode = v
			}
		case resID == attrVersionName || (resID == 0 && name == "versionName"):
			info.VersionName = value(pool, raw, dataType, data)
		}
	}
	if info.Package == "" {
		return Info{}, false, errors.New("atribut package tidak ada pada <manifest>")
	}
	return info, true, nil
}

func value(pool []string, raw uint32, dataType byte, data uint32) string {
	if raw != noIndex && int(raw) < len(pool) {
		return pool[raw]
	}
	switch dataType {
	case typeString:
		return str(pool, data)
	case typeIntDec, typeIntHex:
		return strconv.FormatUint(uint64(data), 10)
	}
	return ""
}

func str(pool []string, idx uint32) string {
	if idx == noIndex || int(idx) >= len(pool) {
		return ""
	}
	return pool[idx]
}

func parseStringPool(c []byte) ([]string, error) {
	if len(c) < 28 {
		return nil, errFormat
	}
	headerSize := int(le16(c, 2))
	count := int(le32(c, 8))
	flags := le32(c, 16)
	stringsStart := int(le32(c, 20))
	if count < 0 || headerSize+count*4 > len(c) || stringsStart > len(c) {
		return nil, errFormat
	}
	utf8Pool := flags&stringPoolUTF8 != 0
	out := make([]string, count)
	for i := 0; i < count; i++ {
		p := stringsStart + int(le32(c, headerSize+i*4))
		if p < 0 || p >= len(c) {
			return nil, errFormat
		}
		if utf8Pool {
			// Panjang karakter lalu panjang byte, masing-masing 1–2 byte.
			_, n, ok := len8(c, p)
			if !ok {
				return nil, errFormat
			}
			p += n
			byteLen, n, ok := len8(c, p)
			if !ok {
				return nil, errFormat
			}
			p += n
			if p+byteLen > len(c) {
				return nil, errFormat
			}
			out[i] = string(c[p : p+byteLen])
			continue
		}
		charLen, n, ok := len16(c, p)
		if !ok {
			return nil, errFormat
		}
		p += n
		if p+charLen*2 > len(c) {
			return nil, errFormat
		}
		units := make([]uint16, charLen)
		for j := range units {
			units[j] = le16(c, p+2*j)
		}
		out[i] = string(utf16.Decode(units))
	}
	return out, nil
}

func len8(b []byte, p int) (int, int, bool) {
	if p >= len(b) {
		return 0, 0, false
	}
	v := int(b[p])
	if v&0x80 == 0 {
		return v, 1, true
	}
	if p+1 >= len(b) {
		return 0, 0, false
	}
	return (v&0x7f)<<8 | int(b[p+1]), 2, true
}

func len16(b []byte, p int) (int, int, bool) {
	if p+2 > len(b) {
		return 0, 0, false
	}
	v := int(le16(b, p))
	if v&0x8000 == 0 {
		return v, 2, true
	}
	if p+4 > len(b) {
		return 0, 0, false
	}
	return (v&0x7fff)<<16 | int(le16(b, p+2)), 4, true
}

func le16(b []byte, off int) uint16 { return binary.LittleEndian.Uint16(b[off:]) }
func le32(b []byte, off int) uint32 { return binary.LittleEndian.Uint32(b[off:]) }
