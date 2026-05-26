//go:build !gui

package main

import "fmt"

func main() {
	fmt.Println("tcforge-gui is built only with the gui build tag.")
}
