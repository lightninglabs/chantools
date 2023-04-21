package btc

import (
	"strings"
)

var (
	inputCharset = "0123456789()[],'/*abcdefgh@:$%{}IJKLMNOPQRSTUVWXYZ" +
		"&+-.;<=>?!^_|~ijklmnopqrstuvwxyzABCDEFGH`#\\\"\\\\ "
	checksumCharset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
	generator       = []uint64{
		0xf5dee51989, 0xa9fdca3312, 0x1bab10e32d, 0x3706b1677a,
		0x644d626ffd,
	}
)

func descriptorSumPolymod(symbols []uint64) uint64 {
	chk := uint64(1)
	for _, value := range symbols {
		top := chk >> 35
		chk = (chk&0x7ffffffff)<<5 ^ value
		for i := 0; i < 5; i++ {
			if (top>>i)&1 != 0 {
				chk ^= generator[i]
			}
		}
	}
	return chk
}

func descriptorSumExpand(s string) []uint64 {
	groups := []uint64{}
	symbols := []uint64{}
	for _, c := range s {
		v := strings.IndexRune(inputCharset, c)
		if v < 0 {
			return nil
		}
		symbols = append(symbols, uint64(v&31))
		groups = append(groups, uint64(v>>5))
		if len(groups) == 3 {
			symbols = append(
				symbols, groups[0]*9+groups[1]*3+groups[2],
			)
			groups = []uint64{}
		}
	}
	if len(groups) == 1 {
		symbols = append(symbols, groups[0])
	} else if len(groups) == 2 {
		symbols = append(symbols, groups[0]*3+groups[1])
	}
	return symbols
}

func DescriptorSumCreate(s string) string {
	symbols := append(descriptorSumExpand(s), 0, 0, 0, 0, 0, 0, 0, 0)
	checksum := descriptorSumPolymod(symbols) ^ 1
	builder := strings.Builder{}
	for i := 0; i < 8; i++ {
		builder.WriteByte(checksumCharset[(checksum>>(5*(7-i)))&31])
	}
	return s + "#" + builder.String()
}

func DescriptorSumCheck(s string, require bool) bool {
	if !strings.Contains(s, "#") {
		return !require
	}
	if s[len(s)-9] != '#' {
		return false
	}
	for _, c := range s[len(s)-8:] {
		if !strings.ContainsRune(checksumCharset, c) {
			return false
		}
	}
	symbols := append(
		descriptorSumExpand(s[:len(s)-9]),
		uint64(strings.Index(checksumCharset, s[len(s)-8:])),
	)
	return descriptorSumPolymod(symbols) == 1
}
