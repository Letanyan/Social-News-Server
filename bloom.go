package main

import (
	"encoding/json"
	"io/ioutil"
	"math"
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

func NewBloomFilterF(fileName string) *BloomFilter {
	content, e := ioutil.ReadFile(fileName)
	if DidFail(e, "read bloom.json file") {
		return &BloomFilter{}
	}

	var result BloomFilter
	e = json.Unmarshal(content, &result)
	if DidFail(e, "unmarshal bloom") {
		return &BloomFilter{}
	}

	return &result
}

func (bloom *BloomFilter) Write(fileName string) {
	file, e := json.Marshal(*bloom)
	if DidFail(e, "marshal bloom") {
		return
	}

	e = ioutil.WriteFile(fileName, file, 0644)
	if DidFail(e, "write bloom to file") {
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

/*

guint32 MurmurHash2(gconstpointer key, gsize len, guint32 seed)
{
	const guint32 m = 0x5bd1e995;
	const gint r = 24;

	guint32 h = seed ^ len;

	const guchar * data = key;
	while(len >= 4)
	{
		guint32 k = *(guint32 *)data;

		k *= m;
		k ^= k >> r;
		k *= m;

		h *= m;
		h ^= k;

		data += 4;
		len -= 4;
	}

	switch(len)
	{
	case 3: h ^= data[2] << 16;
	case 2: h ^= data[1] << 8;
	case 1: h ^= data[0];
		h *= m;
	};

	h ^= h >> 13;
	h *= m;
	h ^= h >> 15;

	return h;
}

*/
