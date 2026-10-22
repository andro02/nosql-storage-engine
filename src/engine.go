package src

import (
	"fmt"
	"strings"

	"github.com/andro02/napredni/config"
)

const helpText = `Available commands:
  PUT <key> <value>          store a value
  GET <key>                  read a value
  DELETE <key>               delete a value
  CMS CREATE <name>          create a Count-Min Sketch
  CMS ADD <name> <value>     add a value to a Count-Min Sketch
  CMS QUERY <name> <value>   estimate how often a value was added
  HLL CREATE <name>          create a HyperLogLog
  HLL ADD <name> <value>     add a value to a HyperLogLog
  HLL COUNT <name>           estimate the number of unique values
  HELP                       show this message`

// cost of a single request in the token bucket
const requestCost = 5

// Engine holds the storage engine state and executes text commands.
// It is shared by the command-line interface and the WebAssembly build.
type Engine struct {
	wal      *Wal
	memtable *Memtable
	cache    *LRUCache
	limiter  *TokenBucket
}

// NewEngine creates an engine using the values loaded from the config file.
func NewEngine() *Engine {
	return &Engine{
		wal:      NewWal(),
		memtable: NewMT(),
		cache:    Init(),
		limiter:  CreateTokenBucket(float64(config.TOKEN_BUCKET_MAX_TOKENS), float64(config.TOKEN_BUCKET_REFILL)),
	}
}

// Execute runs a single command line and prints its result to standard output.
func (engine *Engine) Execute(input string) {
	tokens := strings.Fields(input)
	if len(tokens) == 0 {
		return
	}
	tokens[0] = strings.ToUpper(tokens[0])

	switch tokens[0] {
	case "HELP":
		fmt.Println(helpText)
		return
	case "PUT", "GET", "DELETE", "CMS", "HLL":
	default:
		fmt.Println("Invalid command. Type HELP to see available commands.")
		return
	}

	if !engine.limiter.Request(requestCost) {
		fmt.Println("Too many requests. Please wait a moment.")
		return
	}

	switch tokens[0] {
	case "PUT":
		Put(engine.wal, engine.memtable, tokens)
	case "GET":
		engine.get(tokens)
	case "DELETE":
		Delete(engine.wal, engine.memtable, tokens)
	case "CMS":
		executeCMS(tokens[1:])
	case "HLL":
		executeHLL(tokens[1:])
	}
}

func (engine *Engine) get(tokens []string) {
	value, tombstone := Get(engine.memtable, engine.cache, tokens)
	switch {
	case len(tokens) != 2:
		// Get already reported invalid input
	case tombstone == 1:
		fmt.Println("Key not found (deleted).")
	case value == "Not found":
		fmt.Println("Key not found.")
	default:
		fmt.Println(value)
	}
}

func executeCMS(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: CMS CREATE <name> | CMS ADD <name> <value> | CMS QUERY <name> <value>")
		return
	}

	switch strings.ToUpper(args[0]) {
	case "CREATE":
		if len(args) == 2 {
			CMSCreate(args[1])
			return
		}
	case "ADD":
		if len(args) == 3 {
			CMSAdd(args[1], args[2])
			return
		}
	case "QUERY":
		if len(args) == 3 {
			CMSQuery(args[1], args[2])
			return
		}
	}
	fmt.Println("Invalid input. Type HELP to see available commands.")
}

func executeHLL(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: HLL CREATE <name> | HLL ADD <name> <value> | HLL COUNT <name>")
		return
	}

	switch strings.ToUpper(args[0]) {
	case "CREATE":
		if len(args) == 2 {
			HLLCreate(args[1])
			return
		}
	case "ADD":
		if len(args) == 3 {
			HLLAdd(args[1], args[2])
			return
		}
	case "COUNT":
		if len(args) == 2 {
			HLLCount(args[1])
			return
		}
	}
	fmt.Println("Invalid input. Type HELP to see available commands.")
}
