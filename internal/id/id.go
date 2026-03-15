package id

import (
	"fmt"
	"math/rand/v2"
	"time"

	nanoid "github.com/matoous/go-nanoid/v2"
)

// GenerateID returns a 21-character nanoid, suitable for primary keys.
func GenerateID() (string, error) {
	return nanoid.New()
}

// MustGenerateID returns a 21-character nanoid; panics on failure.
// Use only during initialization, never in request handlers.
func MustGenerateID() string {
	return nanoid.Must()
}

// GenerateOrderNo returns an order number in the format:
// YYYYMMDDHHmmss + 8 random digits (e.g. "2026031511230012345678")
func GenerateOrderNo() string {
	ts := time.Now().Format("20060102150405")
	suffix := fmt.Sprintf("%08d", rand.IntN(100000000))
	return ts + suffix
}
