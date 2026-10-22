package src

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

type CountMinSketch struct {
	d              uint32 //br hash func
	w              uint32 //kolone
	key            [16]byte
	frequencyTable [][]int32
	hashFunctions  []HashWithSeed
	seeds          [][]byte
}

func NewCountMinSketch(key [16]byte, epsilon float64, delta float64) (*CountMinSketch, error) {
	if epsilon <= 0 || delta <= 0 {
		return nil, fmt.Errorf("countminsketch error: values of epsilon and delta should both be greater than 0")
	}

	k := CalculateCols(delta)   // number of hash functions: ceil(ln(e / delta))
	m := CalculateRows(epsilon) // width of each row: ceil(e / epsilon)

	hashFunctions, seeds := GenerateHashFunctions(k)
	cms := &CountMinSketch{
		d:             k,
		w:             m,
		key:           key,
		hashFunctions: hashFunctions,
		seeds:         seeds,
	}
	cms.frequencyTable = make([][]int32, k)

	for r := range cms.frequencyTable {
		cms.frequencyTable[r] = make([]int32, m)
	}

	return cms, nil
}

// func (cms *CountMinSketch) D() uint32{
// 	return cms.d
// }

// func (cms *CountMinSketch) W() uint32{
// 	return cms.w
// }

// CMSCreate creates a new Count-Min Sketch stored under the given name.
func CMSCreate(name string) {
	cms, _ := NewCountMinSketch(ConvertToKey(name), 0.01, 0.01)
	if found, _ := cms.KeyCheck(); found {
		fmt.Println("A CMS with this name already exists.")
		return
	}

	if err := cms.Serialize(); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("CMS created.")
}

// CMSAdd adds a value to an existing Count-Min Sketch.
func CMSAdd(name string, value string) {
	cms := CountMinSketch{key: ConvertToKey(name)}
	found, position := cms.KeyCheck()
	if !found {
		fmt.Println("No CMS with this name.")
		return
	}

	if cms.AddToSketch([]byte(value), position) {
		fmt.Println("Value added to CMS.")
	} else {
		fmt.Println("Something went wrong. Please try again.")
	}
}

// CMSQuery prints the estimated frequency of a value in a Count-Min Sketch.
func CMSQuery(name string, value string) {
	cms := CountMinSketch{key: ConvertToKey(name)}
	if found, _ := cms.KeyCheck(); !found {
		fmt.Println("No CMS with this name.")
		return
	}

	fmt.Printf("Estimated frequency of %q: %d\n", value, cms.GetMinFrequency([]byte(value)))
}

func ConvertToKey(input string) [16]byte {
	max_length := 16
	if len(input) < max_length {
		max_length = len(input)
	}

	key := [16]byte{}
	for i := 0; i < max_length; i++ {
		key[i] = input[i]
	}

	return key
}

func (cms *CountMinSketch) AddToSketch(data []byte, offset uint64) bool {
	for i, hashFunc := range cms.hashFunctions {
		hashedValue := hashFunc.HashAlg(data)
		tableIndex := hashedValue % uint64(cms.w)
		cms.frequencyTable[i][tableIndex] += 1
	}
	cms.Update(offset)
	return true
}

func (cms *CountMinSketch) Update(offset uint64) bool {
	file, err := os.OpenFile("cms.bin", os.O_RDWR, 0600)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer file.Close()

	meta_data, err := file.Stat()
	if err != nil {
		fmt.Println(err)
		return false
	}

	if meta_data.Size() == 0 {
		return false
	}

	data, release, err := mapFile(file, true)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer release()

	cms.FillCMS(data, &offset)

	for i := 0; i < int(cms.d); i++ {
		for j := 0; j < int(cms.w); j++ {
			binary.BigEndian.PutUint32(data[offset:offset+4], uint32(cms.frequencyTable[i][j]))
			offset += 4
		}
	}
	return true
}

func (cms *CountMinSketch) FillCMS(data []byte, offset *uint64) {
	*offset += 16 //skipping key
	cms.w = binary.BigEndian.Uint32(data[*offset : *offset+4])
	*offset += 4

	cms.d = binary.BigEndian.Uint32(data[*offset : *offset+4])
	*offset += 4

}

func (cms *CountMinSketch) GetMinFrequency(data []byte) int32 {
	frequencyValues := make([]int32, cms.d)

	for i, hashFunc := range cms.hashFunctions {
		hashedValue := hashFunc.HashAlg(data)
		tableIndex := hashedValue % uint64(cms.w)

		frequencyValues[i] = cms.frequencyTable[i][tableIndex]
	}

	minFrequency := frequencyValues[0]
	for _, freq := range frequencyValues {
		if freq < minFrequency {
			minFrequency = freq
		}
	}
	return minFrequency
}
func (cms *CountMinSketch) KeyCheck() (bool, uint64) {
	file, err := os.OpenFile("cms.bin", os.O_RDONLY, 0600)
	if err != nil {
		fmt.Println(err)
		return false, 0
	}
	defer file.Close()

	meta_data, err := file.Stat()
	if err != nil {
		fmt.Println(err)
		return false, 0
	}

	if meta_data.Size() == 0 {
		return false, 0
	}

	data, release, err := mapFile(file, false)
	if err != nil {
		fmt.Println(err)
		return false, 0
	}
	defer release()

	var offset uint64
	var current_key [16]byte
	for offset < uint64(len(data)) {
		position := offset
		copy(current_key[:], data[offset:offset+16])

		cms.FillCMS(data, &offset)

		cms.frequencyTable = make([][]int32, cms.d)
		for i := 0; i < int(cms.d); i++ {
			cms.frequencyTable[i] = make([]int32, cms.w)
		}
		if bytes.Equal(current_key[:], cms.key[:]) {
			// If key is wanted one we load cms
			cms.FillTableData(data, &offset)

			return true, position
		} else {
			offset = offset + uint64(cms.d)*uint64(cms.w)*4 + uint64(cms.d)*32
		}
	}
	return false, 0
}

func (cms *CountMinSketch) FillTableData(data []byte, offset *uint64) {
	for i := 0; i < int(cms.d); i++ {
		for j := 0; j < int(cms.w); j++ {
			cms.frequencyTable[i][j] = int32(binary.BigEndian.Uint32(data[*offset : *offset+4]))
			*offset += 4
		}
	}

	cms.seeds = make([][]byte, cms.d)
	for i := 0; i < int(cms.d); i++ {
		cms.seeds[i] = make([]byte, 32)
	}

	for i := 0; i < int(cms.d); i++ {
		copy(cms.seeds[i], data[*offset:*offset+32])
		*offset += 32
	}

	cms.hashFunctions = InitializeHashFunctionsFromSeeds(cms.d, cms.seeds)
}

func (cms *CountMinSketch) Serialize() error {
	file, err := os.OpenFile("cms.bin", os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := cms.Write(file); err != nil {
		return err
	}

	if err := file.Sync(); err != nil {
		return err
	}

	return nil
}

func (cms *CountMinSketch) Write(writer io.Writer) error {
	if err := binary.Write(writer, binary.BigEndian, cms.key); err != nil {
		return err
	}
	if err := binary.Write(writer, binary.BigEndian, cms.w); err != nil {
		return err
	}
	if err := binary.Write(writer, binary.BigEndian, cms.d); err != nil {
		return err
	}

	for i := 0; i < int(cms.d); i++ {
		if err := binary.Write(writer, binary.BigEndian, cms.frequencyTable[i]); err != nil {
			return err
		}
	}
	for i := 0; i < int(cms.d); i++ {
		if err := binary.Write(writer, binary.BigEndian, cms.seeds[i]); err != nil {
			return err
		}
	}

	return nil
}
