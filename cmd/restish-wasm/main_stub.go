//go:build !js || !wasm

package main

import "fmt"

func main() {
	fmt.Println("restish-wasm is a browser WebAssembly target; build with GOOS=js GOARCH=wasm")
}
