package main

import (
	"os"
	"flag"
	"fmt"
	"bufio"
	"strings"
)

// snes-info -query '%{checksum}  %{filename}' ~/Roms/SNES/*.smc | sed -rn "s/^41178  (.*)$/\1/p" | while read line; do if test "$(md5sum "$line") != dbe1f3c8f3a0b2db52b7d59417891117"; then echo "$line"; fi; done

func main() {
	var file *os.File
	var err os.Error
	var snes_error Error
	var fi *os.FileInfo

	var query *string = flag.String("query", "name: %{name}; offset: %{offset}; checksum: %{checksum}", "query some data")
	var query_contains []string
	query_keywords := []string{"%{filename}", "%{name}", "%{offset}", "%{layout}", "%{cart_type}", "%{rom_size}", "%{ram_size}", "%{country_code}", "%{version_number}", "%{checksum}", "%{checksum_complement}", "%{unknown1}", "%{extended}"}

	flag.Parse()

	for _, s := range query_keywords {
		if strings.Contains(*query, s) {
			query_contains = append(query_contains, s)
		}
	}

	n := flag.NArg()

	for i := 0; i < n; i++ {
		path := flag.Arg(i)

		var filename string

		if path == "-" {
//			fmt.Printf("reading from stdin\n")

			file = os.Stdin
			filename = "-"
		} else if path != "" {
//			fmt.Printf("reading file: %s\n", path)

			filename = path

			file, err = os.Open(path, os.O_RDONLY, 0444)
			if err != nil {
				return
			}
		} else if flag.NArg() == 0 {
			fmt.Printf("help\n")
			return
		}

		fi, err = file.Stat()
		if err != nil {
			return
		}

		buf := make([]byte, fi.Size)

		reader := bufio.NewReader(file)
		n,err = reader.Read(buf)
		if err != nil || int64(n) != fi.Size {
			return
		}

		var header_obj snes_header
		header_obj, snes_error = read_snes_header(filename, buf, fi.Size)

		if snes_error > 0 {
			return
		}

		output := *query

		for _, s := range query_contains {
			var val string
			switch s {
				case "%{filename}": val = header_obj.filename
				case "%{name}": val = header_obj.name
				case "%{offset}": val = fmt.Sprintf("0x%x", header_obj.offset)
				case "%{layout}": val = fmt.Sprint(header_obj.layout)
				case "%{cart_type}": val = fmt.Sprint(header_obj.cart_type)
				case "%{rom_size}": val = fmt.Sprint(header_obj.rom_size)
				case "%{ram_size}": val = fmt.Sprint(header_obj.ram_size)
				case "%{country_code}": val = fmt.Sprint(header_obj.country_code)
				case "%{version_number}": val = fmt.Sprint(header_obj.version_number)
				case "%{checksum}": val = fmt.Sprint(header_obj.checksum)
				case "%{checksum_complement}": val = fmt.Sprint(header_obj.checksum_complement)
				case "%{unknown1}": val = fmt.Sprint(header_obj.unknown1)
				case "%{extended}": val = fmt.Sprint(header_obj.extended)
			}
			output = strings.Replace(output, s, val, -1)
		}
		fmt.Printf("%v\n", output)
	}
}

