package src

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andro02/napredni/config"
)

func Get(memtable *Memtable, cache *LRUCache, tokens []string) (string, byte) {

	if len(tokens) != 2 {
		fmt.Println("Invalid input. Please try again.")
		return "", 0
	}

	var key string = tokens[1]

	var foundElement []byte
	if config.MEMTABLE_STRUCTURE == 0 {
		found, i := memtable.BT.SearchKey(key)
		if i != -1 {
			foundElement = found.Data[i].Value
		} else {
			foundElement = nil
		}
	} else {
		foundElement = memtable.SL.SearchElement(key)
	}
	if foundElement != nil {
		entry := WalEntryFromBytes(foundElement)
		return string(entry.Value), entry.Tombstone
	}

	value, cacheFound := cache.Get([]byte(key))
	if cacheFound {
		fmt.Println("Found in cache.")
		return string(value.Value()), value.Tombstone()
	}

	files, _ := os.ReadDir("sstable" + string(filepath.Separator))
	var path string = ""

	for _, toc := range files {
		if strings.Contains(toc.Name(), "toc") {

			file, err := os.Open("sstable" + string(filepath.Separator) + toc.Name())
			if err != nil {
				panic(err)
			}

			scanner := bufio.NewScanner(file)
			scanner.Scan()
			data := scanner.Text()
			path = data[0 : len(data)-8]

			file.Close()

			var value string
			var tombstone byte
			if strings.Contains(data, "data") {
				value, tombstone = Search(key, memtable, cache, path)
			} else {
				value, tombstone = SearchSingle(key, memtable, cache, data)
			}
			if value != "Not found" {
				return value, tombstone
			}
		}
	}

	return "Not found", 0

}

func Search(key string, memtable *Memtable, cache *LRUCache, path string) (string, byte) {

	if SearchBloomFilter(key, path) {
		if path != "" {
			offset, found := SearchSummary(key, path)

			if found {
				offset, found = SearchIndex(key, path, offset)

				if found {
					value, tombstone := GetValueFromDataFile(path, offset)
					cacheInput := &CacheEntry{key: []byte(key), value: []byte(value), timestamp: time.Now().UnixMicro(), tombstone: tombstone}
					cache.Put(cacheInput)
					return value, tombstone
				}
			}
		}
	}

	return "Not found", 0
}

func SearchBloomFilter(key string, path string) bool {
	bloomFilter := Decode(path)
	return bloomFilter.IsInBF(key)
}

func SearchSummary(key string, path string) (uint32, bool) {
	summaryFile, err := os.Open(path + "summary.bin")
	if err != nil {
		panic(err)
	}
	defer summaryFile.Close()
	first := ReadSummaryRow(summaryFile)
	last := ReadSummaryRow(summaryFile)
	if key >= string(first.Key[:]) && key <= string(last.Key[:]) {
		first = ReadSummaryRow(summaryFile)
		for {
			second := ReadSummaryRow(summaryFile)
			if key >= string(first.Key[:]) && key <= string(second.Key[:]) {
				break
			}
			first = second

		}
		return first.Offset, true
	}

	return 0, false
}
func SearchIndex(key string, path string, offset uint32) (uint32, bool) {

	indexFile, err := os.Open(path + "index.bin")
	if err != nil {
		panic(err)
	}
	defer indexFile.Close()
	indexFile.Seek(int64(offset), 0)

	indexEntry, _ := ReadIndexRow(indexFile)
	if string(indexEntry.Key[:]) == key {
		return indexEntry.Offset, true
	}

	for string(indexEntry.Key[:]) < key {
		indexEntry, _ = ReadIndexRow(indexFile)
		if string(indexEntry.Key[:]) == key {
			return indexEntry.Offset, true
		}
	}
	return 0, false

}

func GetValueFromDataFile(path string, offset uint32) (string, byte) {
	dataFile, err := os.Open(path + "data.bin")
	if err != nil {
		panic(err)
	}
	defer dataFile.Close()
	dataFile.Seek(int64(offset), 0)
	dataEntry, _ := ReadWalEntry(dataFile)
	return string(dataEntry.Value), dataEntry.Tombstone

}

func SearchSingle(key string, memtable *Memtable, cache *LRUCache, path string) (string, byte) {

	var foundElement []byte
	if config.MEMTABLE_STRUCTURE == 0 {
		found, i := memtable.BT.SearchKey(key)
		if i != -1 {
			foundElement = found.Data[i].Value
		} else {
			foundElement = nil
		}
	} else {
		foundElement = memtable.SL.SearchElement(key)
	}
	if foundElement != nil {
		entry := WalEntryFromBytes(foundElement)
		return string(entry.Value), entry.Tombstone
	}

	value, cacheFound := cache.Get([]byte(key))
	if cacheFound {
		fmt.Println("Found in cache.")
		return string(value.Value()), value.Tombstone()
	}

	if SearchBloomFilterSingle(key, path) {
		if path != "" {
			offset, found := SearchSummarySingle(key, path)

			if found {
				offset, found = SearchIndexSingle(key, path, offset)

				if found {
					value, tombstone := GetValueFromDataFileSingle(path, offset)
					cacheInput := &CacheEntry{key: []byte(key), value: []byte(value), timestamp: time.Now().UnixMicro(), tombstone: tombstone}
					cache.Put(cacheInput)
					return value, tombstone
				}
			}
		}
	}

	return "Not found", 0
}

func SearchBloomFilterSingle(key string, path string) bool {
	bloomFilter := DecodeSingle(path)
	return bloomFilter.IsInBF(key)
}

func SearchSummarySingle(key string, path string) (uint32, bool) {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	dataSizeBytes := make([]byte, 4)
	_, err = file.Read(dataSizeBytes)
	if err != nil {
		panic(err)
	}
	dataSize := binary.LittleEndian.Uint32(dataSizeBytes)
	file.Seek(int64(dataSize), 1)

	indexSizeBytes := make([]byte, 4)
	_, err = file.Read(indexSizeBytes)
	if err != nil {
		panic(err)
	}
	indexSize := binary.LittleEndian.Uint32(indexSizeBytes)
	file.Seek(int64(indexSize), 1)

	summarySizeBytes := make([]byte, 4)
	_, err = file.Read(summarySizeBytes)
	if err != nil {
		panic(err)
	}
	summarySize := binary.LittleEndian.Uint32(summarySizeBytes)
	file.Seek(int64(summarySize)+4, 1)

	first := ReadSummaryRow(file)
	last := ReadSummaryRow(file)
	if key >= string(first.Key[:]) && key <= string(last.Key[:]) {
		first = ReadSummaryRow(file)
		for {
			second := ReadSummaryRow(file)
			if key >= string(first.Key[:]) && key <= string(second.Key[:]) {
				break
			}
			first = second

		}
		return first.Offset, true
	}

	return 0, false
}

func SearchIndexSingle(key string, path string, offset uint32) (uint32, bool) {

	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	file.Seek(int64(offset), 0)

	indexEntry, _ := ReadIndexRow(file)
	if string(indexEntry.Key[:]) == key {
		return indexEntry.Offset, true
	}

	for string(indexEntry.Key[:]) < key {
		indexEntry, _ = ReadIndexRow(file)
		if string(indexEntry.Key[:]) == key {
			return indexEntry.Offset, true
		}
	}
	return 0, false

}

func GetValueFromDataFileSingle(path string, offset uint32) (string, byte) {
	dataFile, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer dataFile.Close()
	dataFile.Seek(int64(offset), 0)
	dataEntry, _ := ReadWalEntry(dataFile)
	return string(dataEntry.Value), dataEntry.Tombstone

}
