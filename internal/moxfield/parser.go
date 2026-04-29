// Package moxfield provides loaders for deck files in our internal JSON
// format (which is a simplified superset of the Moxfield API export shape).
//
// Future work: integrate with the actual Moxfield public API
// (api.moxfield.com/v3/decks/all/{id}) to import decks by URL.
package moxfield

import (
	"encoding/json"
	"fmt"
	"os"
)

// Deck is the in-memory representation of a Magic deck.
type Deck struct {
	Name      string  `json:"name"`
	Author    string  `json:"author,omitempty"`
	Format    string  `json:"format,omitempty"`
	Commander string  `json:"commander"`
	Mainboard []*Card `json:"mainboard"`
}

// Card is a card entry in a deck. Fields beyond Name and Quantity are
// optional and only used when we want to inspect deck composition without
// hitting the Scryfall oracle layer.
type Card struct {
	Name     string   `json:"name"`
	Quantity int      `json:"quantity"`
	ManaCost string   `json:"mana_cost,omitempty"`
	CMC      int      `json:"cmc,omitempty"`
	Types    []string `json:"types,omitempty"`
	Subtypes []string `json:"subtypes,omitempty"`
	Colors   []string `json:"colors,omitempty"`
	Notes    string   `json:"notes,omitempty"`
}

// LoadDeckFromFile reads a deck JSON file from disk and parses it.
func LoadDeckFromFile(path string) (*Deck, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read deck file %q: %w", path, err)
	}

	var deck Deck
	if err := json.Unmarshal(data, &deck); err != nil {
		return nil, fmt.Errorf("parse deck JSON %q: %w", path, err)
	}

	if err := validate(&deck); err != nil {
		return nil, fmt.Errorf("validate deck %q: %w", path, err)
	}

	return &deck, nil
}

// ExpandLibrary returns the deck's mainboard expanded into individual card
// instances (one entry per copy). For a 99-card commander deck this is the
// pre-shuffle library order. Each instance is a copy of the source Card
// pointer's value (we don't share pointers between instances since each
// instance can be in different zones during a game).
func (d *Deck) ExpandLibrary() []*Card {
	total := 0
	for _, c := range d.Mainboard {
		total += c.Quantity
	}
	library := make([]*Card, 0, total)
	for _, c := range d.Mainboard {
		for i := 0; i < c.Quantity; i++ {
			cardCopy := *c
			cardCopy.Quantity = 1
			library = append(library, &cardCopy)
		}
	}
	return library
}

// CardCount returns the total number of card instances in the mainboard
// (sum of quantities). For a commander deck this should equal 99.
func (d *Deck) CardCount() int {
	total := 0
	for _, c := range d.Mainboard {
		total += c.Quantity
	}
	return total
}

func validate(d *Deck) error {
	if d.Name == "" {
		return fmt.Errorf("deck name is empty")
	}
	if d.Commander == "" {
		return fmt.Errorf("deck has no commander")
	}
	if len(d.Mainboard) == 0 {
		return fmt.Errorf("deck has empty mainboard")
	}
	for i, c := range d.Mainboard {
		if c.Name == "" {
			return fmt.Errorf("mainboard entry %d has empty name", i)
		}
		if c.Quantity < 1 {
			return fmt.Errorf("mainboard entry %q has invalid quantity %d", c.Name, c.Quantity)
		}
	}
	return nil
}
