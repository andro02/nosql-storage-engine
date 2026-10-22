package src

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"math/bits"
	"os"
)

var (
	exp32 = math.Pow(2, 32)
)

type HyperLogLog struct {
	p       uint32 //broj pocetnih bitova iz hesa (precision)
	m       uint32 //broj baketa
	key     [16]byte
	buckets []uint32 //lista baketa
}

func NewHyperLogLog(key [16]byte, precison uint32) *HyperLogLog {
	m, _ := GenerateM(precison)
	hll := &HyperLogLog{
		p:       precison,
		m:       m,
		buckets: make([]uint32, m),
		key:     key,
	}
	return hll

}

func HLLMenu() {
	for {
		fmt.Println("\n1. Kreiranje novog HLL")
		fmt.Println("2. Dodavanje elementa u HLL")
		fmt.Println("3. Provjera elementa u HLL")
		fmt.Println("0. Izlaz")
		fmt.Println("Opcija:")
		var choice int
		n, err := fmt.Scanf("%d\n", &choice)
		if n != 1 || err != nil {
			continue
		}

		switch choice {
		case 0:
			fmt.Println("Izlaz iz HLL menija...")
			return
		case 1:
			CreateHLL()
		case 2:
			AddToHLL()
		case 3:
			CheckHLL()
		default:
			fmt.Println("Neispravan unos. Pokusajte ponovo")
		}
	}
}

func CreateHLL() {
	var input string
	fmt.Println("Unesite kljuc za novi HLL: ")
	fmt.Scanf("%s\n", &input)

	key := ConvertToKey(input)

	hll := NewHyperLogLog(key, 16)
	found, _ := hll.KeyCheck()
	if found {
		fmt.Println("Vec postoji HLL sa tim kljucem.")
	} else {
		// Writing into file
		if err := hll.Serialize(); err == nil {
			fmt.Println("Uspesno ste kreirali HLL.")
		} else {
			fmt.Print(err)
		}
	}
}

func AddToHLL() {
	// Unos i konverzija ključa
	var input string
	fmt.Println("Unesite kljuc HLL-a: ")
	fmt.Scanf("%s\n", &input)

	key := ConvertToKey(input)

	hll := NewHyperLogLog(key, 16)
	ok, pos := hll.KeyCheck()
	if !ok {
		fmt.Println("Ne postoji HLL sa ovim kljucem.")
	} else {
		var val string
		fmt.Println("Unesite vrednost koju zelite da ubacite u HLL: ")
		fmt.Scanf("%s\n", &val)
		hll.Add([]byte(val), pos)
	}
}

func CheckHLL() {
	// Unos i konverzija ključa
	var input string
	fmt.Println("Unesite kljuc HLL-a: ")
	fmt.Scanf("%s\n", &input)

	key := ConvertToKey(input)

	hll := NewHyperLogLog(key, 16)
	ok, _ := hll.KeyCheck()
	if !ok {
		fmt.Println("Ne postoji HLL sa ovim kljucem.")
	} else {
		fmt.Println("Unikatnih elemenata u ovom HLL je: ", hll.CalculateEstimation())
	}
}

func CreateHash(stream []byte) uint32 {
	hash_value := fnv.New32a()
	hash_value.Write(stream)
	sum := hash_value.Sum32()
	hash_value.Reset()
	return sum
}

func GenerateM(p uint32) (uint32, error) {
	if p > 16 || p < 4 {
		return 0, errors.New("precision must be between 4 and 16")
	}
	return uint32(math.Pow(2.0, float64(p))), nil
}

func leftmostActiveBit(hash uint32) uint32 {
	return uint32(1 + bits.LeadingZeros32(hash))
}

func (hll HyperLogLog) Add(data []byte, offset uint64) bool {
	hash_value := CreateHash(data)
	leading_bits := 32 - hll.p                  //broj bitova kada se izuzmu vodece nule
	rest_of_hash := hash_value << hll.p         //odsece vodece 0 iz binarnog zapisa hesa tako sto sve bite pomeri u levo za broj nula
	position := leftmostActiveBit(rest_of_hash) //pozicija prvog levog aktivnog bita je ustvari broj desnih nula koje mu prethode
	index := hash_value >> leading_bits         //pomeri sve bitove u desno i dobije vodece nule

	if position > uint32(hll.buckets[index]) {
		hll.buckets[index] = position
	}

	hll.Update(offset)
	return true
}

func (hll *HyperLogLog) Update(offset uint64) bool {
	file, err := os.OpenFile("hll.bin", os.O_RDWR, 0600)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		fmt.Println(err)
		return false
	}

	if fi.Size() == 0 {
		return false
	}

	data, release, err := mapFile(file, true)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer release()

	offset += 16
	hll.p = binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	hll.m = binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	for i := 0; i < int(hll.m); i++ {
		binary.BigEndian.PutUint32(data[offset:offset+4], uint32(hll.buckets[i]))
		offset += 4
	}

	return true
}

func (hll *HyperLogLog) CountZeros() int {
	var zeros int
	for _, value := range hll.buckets {
		if value == 0 {
			zeros++
		}
	}
	return zeros
}

func (hll *HyperLogLog) Reset() {
	hll.buckets = make([]uint32, hll.m)
}

// procena kardinalnosti
func (hll *HyperLogLog) CalculateEstimation() float64 {
	sum := 0.0
	m := float64(hll.m)
	for _, val := range hll.buckets {
		sum += 1.0 / math.Pow(2.0, float64(val))
	}
	alpha := 0.79402 / (1.0 + 1.079/float64(m))
	estimate := alpha * m * m / sum
	empty_buckets := hll.CountZeros()
	if estimate <= 5.0/2.0*m {
		if empty_buckets > 0 {
			estimate = m * math.Log(m/float64(empty_buckets))
		}
	} else if estimate > 1.0/30.0*exp32 {
		estimate = -exp32 * math.Log(1-estimate/exp32)
	}
	return estimate
}

func (hll *HyperLogLog) KeyCheck() (bool, uint64) {
	file, err := os.OpenFile("hll.bin", os.O_RDONLY, 0600)
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
		hll.FillHLL(data, &offset)

		if bytes.Equal(current_key[:], hll.key[:]) {

			hll.buckets = make([]uint32, hll.m)
			for i := 0; i < int(hll.m); i++ {
				hll.buckets[i] = binary.BigEndian.Uint32(data[offset : offset+4])
				offset += 4
			}

			return true, position
		} else {
			offset = offset + uint64(hll.m)*4
		}
	}
	return false, 0
}

func (hll *HyperLogLog) FillHLL(data []byte, offset *uint64) {
	*offset += 16
	hll.p = binary.BigEndian.Uint32(data[*offset : *offset+4])
	*offset += 4

	hll.m = binary.BigEndian.Uint32(data[*offset : *offset+4])
	*offset += 4

}

func (hll *HyperLogLog) Serialize() error {
	file, err := os.OpenFile("hll.bin", os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := hll.Write(file); err != nil {
		return err
	}

	if err := file.Sync(); err != nil {
		return err
	}

	return nil
}

func (hll *HyperLogLog) Write(writer io.Writer) error {
	var buf bytes.Buffer
	err := binary.Write(&buf, binary.BigEndian, hll.key)
	if err != nil {
		return err
	}
	err = binary.Write(&buf, binary.BigEndian, hll.p)
	if err != nil {
		return err
	}
	err = binary.Write(&buf, binary.BigEndian, hll.m)
	if err != nil {
		return err
	}
	for _, reg := range hll.buckets {
		err = binary.Write(&buf, binary.BigEndian, reg)
		if err != nil {
			return err
		}
	}
	_, err = writer.Write(buf.Bytes())
	return err
}
