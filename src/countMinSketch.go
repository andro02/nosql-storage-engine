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

func CMSMenu() {
	for {
		fmt.Println("\n1. Kreiranje novog CMS")
		fmt.Println("2. Dodavanje elementa u CMS")
		fmt.Println("3. Provjera elementa u CMS")
		fmt.Println("0. Izlaz")
		fmt.Println("Opcija:")
		var choice int
		n, err := fmt.Scanf("%d\n", &choice)
		if n != 1 || err != nil {
			continue
		}

		switch choice {
		case 0:
			fmt.Println("Izlaz iz CMS menija...")
			return
		case 1:
			CreateCMS()
		case 2:
			AddToCMS()
		case 3:
			CheckCMS()
		default:
			fmt.Println("Neispravan unos. Pokusajte ponovo")
		}
	}
}

func CreateCMS() {
	var input string
	fmt.Println("Unesite kljuc za CMS: ")
	fmt.Scanf("%s\n", &input)

	key := ConvertToKey(input)

	cms, _ := NewCountMinSketch(key, 0.01, 0.01)
	found, _ := cms.KeyCheck()
	if found {
		fmt.Println("Već postoji CMS sa tim ključem.")
	} else {
		// Upisivanje u datoteku
		if err := cms.Serialize(); err == nil {
			fmt.Println("Uspesno ste kreirali CMS.")
		} else {
			fmt.Print(err)
		}
	}
}

func AddToCMS() {
	// Unos i konverzija ključa
	var input string
	fmt.Println("Unesite kljuc CMSa: ")
	fmt.Scanf("%s\n", &input)

	key := ConvertToKey(input)

	c := CountMinSketch{key: key}
	ok, pos := c.KeyCheck()
	if !ok {
		fmt.Println("Ne postoji CMS sa ovim ključem.")
	} else {
		var val string
		fmt.Println("Unesite vrednost koju zelite da ubacite u CMS: ")
		fmt.Scanf("%s\n", &val)
		if c.AddToSketch([]byte(val), pos) {
			fmt.Print("Uspjesno ste dodali vrijednost u CMS.")
		} else {
			fmt.Println("Doslo je do greske. Pokusajte ponovo.")
		}
	}
}

func CheckCMS() {
	// Unos i konverzija ključa
	var input string
	fmt.Println("Unesite kljuc CMSa: ")
	fmt.Scanf("%s\n", &input)

	key := ConvertToKey(input)

	c := CountMinSketch{key: key}
	ok, _ := c.KeyCheck()
	if !ok {
		fmt.Println("Ne postoji CMS sa ovim ključem.")
	} else {
		var val string
		fmt.Println("Unesite vrednost koju zelite da proverite: ")
		fmt.Scanln(&val)
		fmt.Println("Pojavljuje se:", c.GetMinFrequency([]byte(val)), "puta.")
	}
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
	fmt.Println(cms.key)
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
