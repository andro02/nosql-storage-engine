//go:build js && wasm

// Command wasm exposes the storage engine to JavaScript when compiled to WebAssembly.
// The page must provide an in-memory file system (globalThis.fs) before starting it.
package main

import (
	"fmt"
	"syscall/js"

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

	// kvExecute(line) runs one command and returns a Promise that resolves when it is done;
	// output is written to standard output, which the page captures
	js.Global().Set("kvExecute", js.FuncOf(func(this js.Value, args []js.Value) any {
		input := args[0].String()

		executor := js.FuncOf(func(this js.Value, callbacks []js.Value) any {
			resolve, reject := callbacks[0], callbacks[1]

			// file operations wait for JavaScript callbacks, so the command must run
			// in its own goroutine instead of blocking this callback
			go func() {
				defer func() {
					if r := recover(); r != nil {
						reject.Invoke(fmt.Sprint(r))
					}
				}()
				engine.Execute(input)
				resolve.Invoke()
			}()
			return nil
		})
		defer executor.Release()

		return js.Global().Get("Promise").New(executor)
	}))

	// tell the page the engine is ready to accept commands
	if onReady := js.Global().Get("kvOnReady"); onReady.Type() == js.TypeFunction {
		onReady.Invoke()
	}

	// keep the Go runtime alive so kvExecute can be called at any time
	select {}
}
