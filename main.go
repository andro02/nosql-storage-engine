package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"github.com/andro02/napredni/config"
	"github.com/andro02/napredni/src"
)

func main() {
	cfg, err := config.ReadConfig("config.txt")
	if err != nil {
		panic(err)
	}
	config.LoadValues(cfg)

	engine := src.NewEngine()
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print(">>> ")
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				log.Fatal(err)
			}
			return
		}
		engine.Execute(scanner.Text())
	}
}
