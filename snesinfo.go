package main

// http://romhack.wikia.com/wiki/SMC_header
// http://romhack.wikia.com/wiki/SNES_header

import (
	"math"
	"bytes"
	"encoding/binary"

//	"unsafe"
)

const (
	ROM_NAME_LEN = 23

	// layout type
	LOROM = 1 << 0
	HIROM = 1 << 1
	FAST = 1 << 2

	// cart type
	ROM = 1 << 0
	RAM = 1 << 1
	BATTERY = 1 << 2
	SA1 = 1 << 3
	SUPERFX = 1 << 4

	// country code
	INVALID = 0
	NTSC = 1
	PAL = 2
)

const (
	ERR_OFFSET = 0
)

var (
	codes = map[int]string{ERR_OFFSET: "offset could not be found"}
)

type Error int
func (e *Error) String() string { return string(codes[int(*e)]) }

type snes_header struct {
	filename string
	offset int
	name string
	layout uint8
	cart_type uint8
	rom_size uint32 // in kilobytes
	ram_size uint32 // in kilobytes
	country_code uint8
	licensee_code uint8
	version_number uint8
	checksum uint16
	checksum_complement uint16
	unknown1 uint32
	extended string
}

func allASCII(start int, b []byte, size int) bool {
	for i := 0; i < size; i++ {
		if b[start + i] < 32 || b[start + i] > 126 {
			return false
		}
	}
	return true
}

func hirom_likelyhood(buf []byte, calculated_size int64, headerless bool) int {
	score := 0
	var offset int
	if headerless {
		offset = 0xff00
	} else {
		offset = 0xff00 + 0x200
	}

	if buf[0xd5 + offset] & 0x1 != 0 {
		score += 2
	}

	// Mode23 is SA-1
	if (buf[0xd5 + offset] == 0x23) {
		score -= 2
	}

	if (buf[0xd4 + offset] == 0x20) {
		score += 2
	}

	if buf[0xdd + offset] + buf[0xdf + offset] == 0xff && buf[0xdc + offset] + buf[0xde + offset] == 0xff {
		score += 2
		if 0 != buf[0xde + offset] && 0 != buf[0xdf + offset]{
			score++
		}
	}

	if (buf[0xda + offset] == 0x33) {
		score += 2
	}

	if ((buf[0xd5 + offset] & 0xf) < 4) {
		score += 2
	}

	if buf[0xfd + offset] & 0x80 == 0 {
		score -= 6
	}

	if buf[0xfd + offset] > 0xff && buf[0xfc + offset] > 0xb0 {
		score -= 2
	}

	if calculated_size > 1024 * 1024 * 3 {
		score += 4
	}

	if (1 << (buf[0xd7 + offset] - 7)) > 48 {
		score -= 1
	}

	if !allASCII(0xb0 + offset, buf, 6) {
		score -= 1
	}

	if !allASCII(0xc0 + offset, buf, ROM_NAME_LEN - 1) {
		score -= 1
	}

	return score
}

func lorom_likelyhood(buf []byte, calculated_size int64, headerless bool) int {
	score := 0
	var offset int
	if headerless {
		offset = 0x7f00
	} else {
		offset = 0x7f00 + 0x200
	}

	if buf[0xd5 + offset] & 0x1 == 0 {
		score += 3
	}

	if buf[0xd5 + offset] == 0x23 {
		score += 2
	}

	if buf[0xdc + offset] + buf[0xde + offset] == 0xff && buf[0xdd + offset] + buf[0xdf + offset] == 0xff {
		score += 2
		if 0 != buf[0xde + offset] && 0 != buf[0xdf + offset] {
			score++
		}
	}

	if buf[0xda + offset] == 0x33 {
		score += 2
	}

	if (buf[0xd5 + offset] & 0xf) < 4 {
		score += 2
	}

	if buf[0xfd + offset] & 0x80 == 0 {
		score -= 6
	}

	if buf[0xfc + offset] > 0xb0 && buf[0xfd + offset] > 0xb0 {
		score -= 2
	}

	if calculated_size <= 1024 * 1024 * 16 {
		score += 2
	}

	if (1 << (buf[0xd7 + offset] - 7)) > 48 {
		score -= 1
	}

	if !allASCII(0xb0 + offset, buf, 6) {
		score -= 1
	}

	if !allASCII(0xc0 + offset, buf, ROM_NAME_LEN - 1) {
		score -= 1
	}

	return score
}

func get_offset(buf []byte, calculated_size int64) (offset int, err Error) {
	var headerless bool
	if calculated_size % 1024 == 512 {
		headerless = false
	} else if calculated_size % 1024 == 0 {
		headerless= true
	} else {
		//fmt.Fprintf(os.Stdout, "the file appears to be corrupt\n")
		return 0, Error(ERR_OFFSET)
	}

	lorom_score := lorom_likelyhood(buf, calculated_size, headerless)
	hirom_score := hirom_likelyhood(buf, calculated_size, headerless)

	if lorom_score > hirom_score {
		offset = 0x7f00
	} else if lorom_score < hirom_score {
		offset = 0xff00
	}

	if !headerless {
		offset += 0x200
	}

	return offset, err
}

func read_snes_header(filename string, b []byte, size int64) (p snes_header, err Error) {
	var offset int
	offset, err = get_offset(b, size)
	if err > 0 {
		return p, err
	}

	p.offset = offset
	p.filename = filename

	name := make([]byte, ROM_NAME_LEN)
	for i := 0; i < ROM_NAME_LEN - 2; i++ {
		if (b[i + 0xc0 + offset] > 32 && b[i + 0xc0 + offset] < 126) || (b[i + 0xc0 + offset] == 0x20 && b[i + 0xc0 + offset + 1] != 0x20) {
			name[i] = b[i + 0xc0 + offset]
		} else {
			break
		}
	}

	buf := bytes.NewBuffer(name)
	p.name = buf.String()

	p.layout = 0
	if b[0xd5 + offset] & 0x01 == 0x01 {
		p.layout |= HIROM
	} else {
		p.layout |= LOROM
	}

	if b[0xd5 + offset] & 0x10 == 0x10 {
		p.layout |= FAST
	}

	switch b[0xd6 + offset] {
		case 0x00: p.cart_type = ROM
		case 0x01: p.cart_type = ROM | RAM
		case 0x02: p.cart_type = ROM | RAM | BATTERY
		case 0x13: p.cart_type = ROM | SUPERFX
		case 0x14: p.cart_type = ROM | SUPERFX
		case 0x15: p.cart_type = ROM | RAM | SUPERFX
		case 0x1a: p.cart_type = ROM | RAM | BATTERY | SUPERFX
		case 0x34: p.cart_type = ROM | RAM | SA1
		case 0x35: p.cart_type = ROM | RAM | BATTERY | SA1
	}

	p.rom_size = uint32(math.Pow(2, float64(b[0xd7 + offset])))
	p.ram_size = uint32(math.Pow(2, float64(b[0xd8 + offset])))

	if b[0xd9 + offset] == 0x00 || b[0xd9 + offset] == 0x01 || b[0xd9 + offset] == 0x0d {
		p.country_code = NTSC
	} else if b[0xd9 + offset] >= 02 && b[0xd9 + offset] <= 0x0c {
		p.country_code = PAL
	} else {
		p.country_code = INVALID
	}

	p.licensee_code = b[0xda + offset]
	p.version_number = b[0xdb + offset]

	p.checksum_complement = binary.LittleEndian.Uint16([]uint8{b[0xdc + offset], b[0xdd + offset]})
	p.checksum = binary.LittleEndian.Uint16([]uint8{b[0xde + offset], b[0xdf + offset]})

	p.unknown1 = uint32(b[0xe0 + offset] << 24 | b[0xe1 + offset] << 16 | b[0xe2 + offset] << 8 | b[0xe3 + offset])

	if allASCII(offset + 0xb2, b, 4) {
		extended := make([]byte, ROM_NAME_LEN)
		for i := 0; i < ROM_NAME_LEN - 2; i++ {
			if (b[i + 0xb2 + offset] > 32 && b[i + 0xc0 + offset] < 126) || b[i + 0xb2 + offset] == 0x20 {
				extended[i] = b[i + 0xb2 + offset]
			} else {
				break
			}
		}
		buf := bytes.NewBuffer(extended)
		p.extended = buf.String()
	}

	return p, err
}

