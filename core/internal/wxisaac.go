package internal

import (
	"encoding/binary"
	"slices"
	"strconv"

	"github.com/lbbniu/isaac"
)

const KeyLen = 131072

func GetDecryptorBytes(decryptKey string) (keys []byte) {
	seed, _ := strconv.ParseUint(decryptKey, 10, 64)
	s := isaac.New[uint64]()
	var seeds [isaac.Words]uint64
	seeds[0] = seed
	s.Seed(seeds)
	var results [isaac.Words]uint64
	for i := 0; i < KeyLen/isaac.Words/isaac.WordsLog; i++ {
		s.Refill(&results)
		rs := results[:]
		slices.Reverse(rs)
		key := make([]byte, isaac.WordsLog)
		for _, r := range rs {
			binary.BigEndian.PutUint64(key, r)
			keys = append(keys, key...)
		}
	}
	return keys
}
