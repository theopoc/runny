// Prototype jetable: trois variantes de refonte TUI, servies sur une route unique.
package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
)

//go:embed index.html
var assets embed.FS

func main() {
	const address = "127.0.0.1:8787"
	fmt.Printf("Runny TUI prototype: http://%s/?variant=A\n", address)
	log.Fatal(http.ListenAndServe(address, http.FileServer(http.FS(assets))))
}
