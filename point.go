// Copyright (C) 2016 JT Olds
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dkdtree

import (
	"bytes"
	"encoding/binary"
	"io"
)

const (
	// these assumptions are coded into serialization version 0
	float64Size = 8
	uint32Size  = 4
	uint64Size  = 8
)

func init() {
	if float64Size != binary.Size(float64(0)) ||
		uint32Size != binary.Size(uint32(0)) ||
		uint64Size != binary.Size(uint64(0)) {
		panic("uh oh")
	}
}

func pointSize(dims, maxDataLen int) int {
	return 1 + uint32Size*3 + dims*float64Size + maxDataLen
}

type Point struct {
	Pos  []float64
	Data []byte
}

func (p1 *Point) equal(p2 *Point) bool {
	if len(p1.Pos) != len(p2.Pos) ||
		len(p1.Data) != len(p2.Data) {
		return false
	}
	for i, f1 := range p1.Pos {
		if p2.Pos[i] != f1 {
			return false
		}
	}
	return bytes.Equal(p1.Data, p2.Data)
}

func (p1 *Point) distanceSquared(p2 *Point) (sum float64) {
	for i, v := range p1.Pos {
		delta := v - p2.Pos[i]
		sum += delta * delta
	}
	return sum
}

func (p *Point) serialize(w io.Writer, maxDataLen int) error {
	if len(p.Data) > maxDataLen {
		return errClass.New("data length (%d) greater than max data length (%d)",
			len(p.Data), maxDataLen)
	}
	// serialization version
	_, err := w.Write([]byte{0})
	if err != nil {
		return errClass.Wrap(err)
	}
	// number of floating point values
	posLen := uint32(len(p.Pos))
	err = binary.Write(w, binary.LittleEndian, posLen)
	if err != nil {
		return errClass.Wrap(err)
	}
	// number of data bytes
	dataLen := uint32(len(p.Data))
	err = binary.Write(w, binary.LittleEndian, dataLen)
	if err != nil {
		return errClass.Wrap(err)
	}
	// padding
	paddingLen := uint32(maxDataLen - len(p.Data))
	err = binary.Write(w, binary.LittleEndian, paddingLen)
	if err != nil {
		return errClass.Wrap(err)
	}
	// floating point values
	err = binary.Write(w, binary.LittleEndian, p.Pos)
	if err != nil {
		return errClass.Wrap(err)
	}
	// data
	_, err = w.Write(p.Data)
	if err != nil {
		return errClass.Wrap(err)
	}
	// padding
	_, err = w.Write(make([]byte, paddingLen))
	return errClass.Wrap(err)
}

func parsePointHeader(buf []byte) (dims, datalen, padlen uint32,
	remaining []byte, err error) {
	if buf[0] != 0 {
		return 0, 0, 0, nil, errClass.New("invalid serialization version")
	}
	buf = buf[1:]

	dims = binary.LittleEndian.Uint32(buf)
	buf = buf[uint32Size:]
	datalen = binary.LittleEndian.Uint32(buf)
	buf = buf[uint32Size:]
	padlen = binary.LittleEndian.Uint32(buf)
	buf = buf[uint32Size:]
	return dims, datalen, padlen, buf, nil
}

func parsePoint(buf []byte) (rv Point, remaining []byte, err error) {
	dims, datalen, padlen, body, err := parsePointHeader(buf)
	if err != nil {
		return rv, nil, err
	}

	posBytes := dims * float64Size

	rv.Pos, err = readFloats(body[:posBytes])
	if err != nil {
		return rv, nil, errClass.Wrap(err)
	}
	body = body[posBytes:]

	rv.Data = body[:datalen]

	return rv, body[datalen+padlen:], nil
}

func parsePointFromReader(r io.Reader) (rv Point, maxDataLen int, err error) {
	var header [1 + 3*uint32Size]byte
	_, err = io.ReadFull(r, header[:])
	if err != nil {
		return rv, 0, err
	}
	dims, datalen, padlen, _, err := parsePointHeader(header[:])
	if err != nil {
		return rv, 0, err
	}

	data := make([]byte, len(header)+int(dims)*float64Size+int(datalen+padlen))
	copy(data, header[:])
	_, err = io.ReadFull(r, data[len(header):])
	if err != nil {
		return rv, 0, err
	}
	rv, _, err = parsePoint(data)
	return rv, int(datalen + padlen), err
}
