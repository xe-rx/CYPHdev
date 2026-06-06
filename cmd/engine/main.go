package main

import (
	"context"
	"crypto/ed25519"
	"os"
	"os/signal"
	"strings"
	"time"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"log"

	"github.com/xe-rx/CYPHdev/engine"
	"gopkg.in/yaml.v3"
)

func main() {
	profilePath := flag.String("profile", "configs/profiles/dynamos.yaml", "prefix profile YAML")
	endpoints := flag.String("endpoints", "localhost:2379", "comma separated etcd endpoints")
	dataDir := flag.String("data", "data", "data directory for the append only logs")
	keyPath := flag.String("key", "cyph.key", "Ed25519 private key file (created if absent)")
	ckptEvery := flag.Int("checkpoint every", 16, "sign a checkpoint after this many events")
	ckptInterval := flag.Duration("checkpoint interval", 10*time.Second, "or at least this often")
	flag.Parse()

	prefixes, err := loadPrefixes(*profilePath)
	if err != nil {
		log.Fatalf("profile: %v", err)
	}
	priv, err := loadOrCreateKey(*keyPath)
	if err != nil {
		log.Fatalf("key: %v", err)
	}

	eng, err := engine.New(engine.Config{
		Endpoints:          strings.Split(*endpoints, ","),
		Prefixes:           prefixes,
		DataDir:            *dataDir,
		CheckpointEvery:    *ckptEvery,
		CheckpointInterval: *ckptInterval,
	}, priv)
	if err != nil {
		log.Fatalf("engine: %v", err)
	}
	defer eng.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	log.Printf("cyph engine: watching %d prefixes, committing to %s", len(prefixes), *dataDir)
	if err := eng.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("engine: %v", err)
	}
}

func loadPrefixes(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p struct {
		Prefixes []string `yaml:"prefixes"`
	}
	if err := yaml.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	return p.Prefixes, nil
}

func loadOrCreateKey(path string) (ed25519.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		raw, err := hex.DecodeString(strings.TrimSpace(string(b)))
		if err != nil {
			return nil, err
		}
		if len(raw) != ed25519.PrivateKeySize {
			return nil, errors.New("unexpected key size")
		}
		return ed25519.PrivateKey(raw), nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(hex.EncodeToString(priv)), 0o600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path+".pub", []byte(hex.EncodeToString(pub)), 0o644); err != nil {
		return nil, err
	}
	return priv, nil
}
