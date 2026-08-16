package main

import "github.com/alexzimmer96/exonex/internal/cortex"

func main() {
	srv := cortex.NewServer(":9000")
	srv.ListenAndServe()
}
