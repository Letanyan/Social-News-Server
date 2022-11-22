package main

import (
	"encoding/gob"
	"math"
	"os"
)

type BloomFilter struct {
	HashFunctionCount uint32
	BitCount          uint32
	Buckets           []uint64
}

func BloomP(probability float64, numberOfElements int) (uint32, uint32) {
	m := -(float64(numberOfElements) * math.Log(probability)) / math.Pow(math.Log(2.0), 2.0)
	k := math.Log(2.0) * m / float64(numberOfElements)
	bitCount := uint32(math.Round(m + 0.5))
	hashCount := uint32(math.Round(k + 0.5))
	return bitCount, hashCount
}

func NewBloomFilterP(probability float64, numberOfElements int) *BloomFilter {
	bitCount, hashCount := BloomP(probability, numberOfElements)
	data := []uint64{}
	for i := uint32(0); i < bitCount; i += 64 {
		data = append(data, 0)
	}
	return &BloomFilter{hashCount, bitCount, data}
}

func (bloom *BloomFilter) Insert(word string) {
	for i := uint32(0); i < bloom.HashFunctionCount; i += 1 {
		hash := MurmurHash(word, i)
		pos := hash % bloom.BitCount
		slot := pos / 64
		bit := pos % 64
		bloom.Buckets[slot] |= uint64(1) << uint64(bit)
	}
}

func (bloom *BloomFilter) Contains(word string) bool {
	for i := uint32(0); i < bloom.HashFunctionCount; i += 1 {
		hash := MurmurHash(word, i)
		pos := hash % bloom.BitCount
		slot := pos / 64
		bit := pos % 64
		if bloom.Buckets[slot]&(uint64(1)<<uint64(bit)) == 0 {
			return false
		}
	}
	return true
}

func (bloom *BloomFilter) Clear() {
	for i := range bloom.Buckets {
		bloom.Buckets[i] = 0
	}
}

func NewBloomFilterF(fileName string) *BloomFilter {
	result := BloomFilter{}
	file, _ := os.Open(fileName)
	defer file.Close()
	dec := gob.NewDecoder(file)
	err := dec.Decode(&result)
	if err != nil {
		print(err.Error())
	}
	return &result
}

func (bloom *BloomFilter) Write(fileName string) {
	file, _ := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 0644)
	defer file.Close()
	enc := gob.NewEncoder(file)
	err := enc.Encode(*bloom)
	if DidFail(err, "gob write file") {
		print(err.Error())
		return
	}
}

func HashSetFromFile[V any](fileName string) map[string]V {
	result := map[string]V{}
	file, e := os.Open(fileName)
	if DidFail(e, "open file ", fileName) {
		return result
	}
	defer file.Close()

	dec := gob.NewDecoder(file)
	e = dec.Decode(&result)
	if DidFail(e, "decode file ", fileName, " to hash map") {
		return result
	}
	return result
}

func HashSetWriteToFile[V any](hashSet map[string]V, fileName string) {
	file, e := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 0644)
	if DidFail(e, "open file ", fileName) {
		return
	}
	defer file.Close()
	enc := gob.NewEncoder(file)
	e = enc.Encode(hashSet)
	if DidFail(e, "gob write file") {
		return
	}
}

func MurmurHash(word string, seed uint32) uint32 {
	m := uint32(0x5bd1e995)
	r := int32(24)

	h := seed ^ uint32(len(word))

	data := []byte(word)
	i := 0
	k := uint32(0)
	for i < len(data) {
		k = k<<8 | uint32(data[i])
		i += 1
		if i%4 == 0 {
			k *= m
			k ^= k >> r
			k *= m

			h *= m
			h ^= k

			k = 0
		}
	}

	switch len(data) % 4 {
	case 3:
		h ^= uint32(data[len(data)-3]) << 16
		fallthrough
	case 2:
		h ^= uint32(data[len(data)-2]) << 8
		fallthrough
	case 1:
		h ^= uint32(data[len(data)-1])
		h *= m
	}

	h ^= h >> 13
	h *= m
	h ^= h >> 15

	return h
}
