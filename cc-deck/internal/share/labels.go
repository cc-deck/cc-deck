package share

import (
	"crypto/rand"
	"fmt"
	"io"
)

var labelAdjectives = []string{
	"brave", "bright", "calm", "clever", "eager", "gentle", "happy", "kind",
	"lively", "merry", "nimble", "proud", "quiet", "swift", "warm", "wise",
}

var labelNouns = []string{
	"otter", "panda", "badger", "falcon", "gecko", "heron", "koala", "lynx",
	"moose", "newt", "owl", "quail", "raven", "seal", "tiger", "wolf",
}

type LabelGenerator struct {
	random io.Reader
}

func NewLabelGenerator(random io.Reader) *LabelGenerator {
	if random == nil {
		random = rand.Reader
	}
	return &LabelGenerator{random: random}
}

func (g *LabelGenerator) Next(existing map[string]bool) (string, error) {
	var pair [2]byte
	for range 64 {
		if _, err := io.ReadFull(g.random, pair[:]); err != nil {
			return "", fmt.Errorf("generate unique invitation label: %w", err)
		}
		label := labelAdjectives[int(pair[0])%len(labelAdjectives)] + "-" + labelNouns[int(pair[1])%len(labelNouns)]
		if !existing[label] {
			return label, nil
		}
	}
	return "", fmt.Errorf("generate unique invitation label: exhausted 64 attempts")
}
