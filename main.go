package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/exp/mmap"
)

func ChangeFileExt(filename, newExt string) string {
	base := filename[:len(filename)-len(filepath.Ext(filename))]
	return base + newExt
}

func readFields(fileName string) []string {

	const restBytes = int64(1024 * 4)

	f, err := os.OpenFile(fileName, os.O_RDONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		panic(err)
	}

	buf := make([]byte, restBytes)
	if fi.Size() > restBytes {
		f.Seek(-restBytes, 2)
		f.Read(buf)
	} else {
		f.Seek(0, 0)
		f.Read(buf)
	}

	fields := []string{}

	re := regexp.MustCompile(`{(?m)("[^"]+"),"\S*",\S*,\s*{"Pattern",`)

	matches := re.FindAllStringSubmatch(string(buf), -1)

	for _, match := range matches {
		fields = append(fields, match[1])
	}

	return fields
}

func processFile(filename string) error {

	fieldNames := readFields(filename)

	if len(fieldNames) == 0 {
		panic("формат файла не поддерживается")
	}

	m, err := mmap.Open(filename)
	if err != nil {
		return fmt.Errorf("ошибка при открытии файла %q: %v", filename, err)
	}
	defer m.Close()

	var p int64
	buf := make([]byte, 3, 1024*16)
	for {
		_, err = m.ReadAt(buf, p)
		if err != nil {
			break
		}
		if string(buf) == "\"T\"" {
			p += 3

			break
		}
		p++
	}

	if err != nil {
		if err == io.EOF {
			panic("неправильный формат файла")
		} else {
			panic(err)
		}
	}

	buf = buf[:0]
	p++
	b := m.At(int(p))
	for b >= '0' && b <= '9' {
		buf = append(buf, b)
		p++
		b = m.At(int(p))
	}
	rows, err := strconv.Atoi(string(buf))
	if err != nil {
		panic(err)
	}
	buf = buf[:0]
	p++
	b = m.At(int(p))
	for b >= '0' && b <= '9' {
		buf = append(buf, b)
		p++
		b = m.At(int(p))
	}
	cols, err := strconv.Atoi(string(buf))
	if err != nil {
		panic(err)
	}

	outfilename := ChangeFileExt(filename, ".csv")
	file, err := os.Create(outfilename)
	if err != nil {
		panic("ошибка при создании файла")
	}
	defer file.Close()

	out := bufio.NewWriter(file)

	for r := range rows {
		buf = buf[:0]
		for col := range cols {
			p++
			b = m.At(int(p))
			for b != ',' {
				p++
				b = m.At(int(p))
			}
			p++
			b = m.At(int(p))
			if b == '"' {
				buf = append(buf, b)
				p++
				b = m.At(int(p))
				inq := true
				for {
					if b == '"' {
						if m.At(int(p+1)) == '"' {
							buf = append(buf, '"')
							p++
						} else {
							buf = append(buf, '"')
							inq = false
							break
						}
					}
					buf = append(buf, b)
					p++
					b = m.At(int(p))
					if b == ',' && !inq {
						break
					}
				}

			} else {
				for b != ',' {
					buf = append(buf, b)
					p++
					b = m.At(int(p))
				}
			}
			if col < cols-1 {
				buf = append(buf, ';')
			}
			p++
		}

		if r == 0 {
			buf = buf[:0]
			s := strings.Join(fieldNames, ";")
			fmt.Println(fieldNames)
			buf = append(buf, []byte(s)...)
		}

		buf = append(buf, '\n')
		_, err = out.Write(buf)
		if err != nil {
			panic("ошибка записи")
		}
	}

	err = out.Flush()
	if err != nil {
		panic("ошибка при сбросе буфера")
	}

	return nil
}

func main() {

	if len(os.Args) < 2 {
		fmt.Println("Укажите хотя бы одно имя файла как аргумент.")
		os.Exit(1)
	}

	for _, filename := range os.Args[1:] {
		err := processFile(filename)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
	}
}
