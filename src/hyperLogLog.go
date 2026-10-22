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

// HLLCreate creates a new HyperLogLog stored under the given name.
func HLLCreate(name string) {
	hll := NewHyperLogLog(ConvertToKey(name), 16)
	if found, _ := hll.KeyCheck(); found {
		fmt.Println("An HLL with this name already exists.")
		return
	}

	if err := hll.Serialize(); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("HLL created.")
}

// HLLAdd adds a value to an existing HyperLogLog.
func HLLAdd(name string, value string) {
	hll := NewHyperLogLog(ConvertToKey(name), 16)
	found, position := hll.KeyCheck()
	if !found {
		fmt.Println("No HLL with this name.")
		return
	}

	hll.Add([]byte(value), position)
	fmt.Println("Value added to HLL.")
}

// HLLCount prints the estimated number of unique values in a HyperLogLog.
func HLLCount(name string) {
	hll := NewHyperLogLog(ConvertToKey(name), 16)
	if found, _ := hll.KeyCheck(); !found {
		fmt.Println("No HLL with this name.")
		return
	}

	fmt.Printf("Estimated number of unique values: %.0f\n", hll.CalculateEstimation())
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
