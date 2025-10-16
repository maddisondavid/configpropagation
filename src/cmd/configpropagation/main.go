package main

import (
	"log"
	"time"
)

func main() {
	log.Println("configpropagation operator binary is not yet wired to Kubernetes; blocking until exit")
	for {
		time.Sleep(24 * time.Hour)
	}
}
