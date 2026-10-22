package src

import "fmt"

func Delete(wal *Wal, memtable *Memtable, tokens []string) {

	if len(tokens) != 2 {
		fmt.Println("Invalid input. Please try again.")
		return
	}

	var key string = tokens[1]

	walEntry := wal.Write(key, []byte{}, 1)
	walEntry.Tombstone = 1
	memtable.Delete(key, walEntry.ToBytes())

}
