// Winicon writes the slot machine icon as a Windows .ico file, for the
// release build to link into the exe so that Explorer shows it:
//
//	go run ./cmd/winicon videocasino.ico
//
// The icon is drawn by ui.Icon, the same as the window's. Each size is
// kept in the file as a PNG, which Windows has read since Vista.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image/png"
	"log"
	"os"

	"videocasino/ui"
)

// sizes are the sizes in the file: the window's, and larger ones for
// Explorer's bigger views.
var sizes = []int{16, 32, 48, 64, 128, 256}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: winicon file.ico")
		os.Exit(2)
	}
	ico, err := makeIco()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(os.Args[1], ico, 0o644); err != nil {
		log.Fatal(err)
	}
}

// makeIco lays out the file: a header, an entry for each size, then the
// images.
func makeIco() ([]byte, error) {
	var images [][]byte
	for _, size := range sizes {
		var b bytes.Buffer
		if err := png.Encode(&b, ui.Icon(size)); err != nil {
			return nil, err
		}
		images = append(images, b.Bytes())
	}

	le := binary.LittleEndian
	var out []byte
	out = le.AppendUint16(out, 0) // reserved
	out = le.AppendUint16(out, 1) // an icon, not a cursor
	out = le.AppendUint16(out, uint16(len(sizes)))

	offset := 6 + 16*len(sizes)
	for i, size := range sizes {
		side := byte(size) // 256 is written as 0
		out = append(out, side, side, 0, 0)
		out = le.AppendUint16(out, 1)  // colour planes
		out = le.AppendUint16(out, 32) // bits a pixel
		out = le.AppendUint32(out, uint32(len(images[i])))
		out = le.AppendUint32(out, uint32(offset))
		offset += len(images[i])
	}
	for _, img := range images {
		out = append(out, img...)
	}
	return out, nil
}
