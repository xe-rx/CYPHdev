package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"strings"

	"log"
	"os"

	"github.com/xe-rx/CYPHdev/verify"
)

func main() {
	dir := flag.String("data", "data", "data directory holding the append only logs")
	pubPath := flag.String("pub", "cyph.key.pub", "CYPH Ed25519 public key file (hex)")
	flag.Parse()

	raw, err := os.ReadFile(*pubPath)
	if err != nil {
		log.Fatalf("public key: %v", err)
	}
	key, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(key) != ed25519.PublicKeySize {
		log.Fatalf("public key: malformed (want %d hex encoded bytes)", ed25519.PublicKeySize)
	}

	res, err := verify.Log(*dir, ed25519.PublicKey(key))
	if err != nil {
		log.Fatalf("INVALID! %v", err)
	}
	log.Printf("OK: %d blocks, %d events, %d gaps, %d signed checkpoints verified",
		res.Blocks, res.Events, res.Gaps, res.Checkpoints)
}
