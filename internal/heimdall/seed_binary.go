package heimdall

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"os"
	"sync"
)

const (
	SeedRecordSize = 28
	seedMagic      = 0x48454944 // "HEID" — file header magic
	seedVersion    = 1
	seedHeaderSize = 8 // 4 bytes magic + 4 bytes version

	maxSeedFileSize = 100 * 1024 * 1024 // 100MB rotation threshold
)

// Kill method enum (1 byte). APPEND-ONLY: values are persisted in
// binary seed records — never renumber. KillConcession/KillDraw added
// in consolidation step 2 when ClassifyKill gained those outputs (a
// concession-closed game previously misread as "mill"); records written
// before then never contain 7/8.
const (
	KillCombat     uint8 = 0
	KillCommander  uint8 = 1
	KillCombo      uint8 = 2
	KillMill       uint8 = 3
	KillPoison     uint8 = 4
	KillTimeout    uint8 = 5
	KillUnknown    uint8 = 6
	KillConcession uint8 = 7
	KillDraw       uint8 = 8
)

var killMethodToEnum = map[string]uint8{
	"combat":     KillCombat,
	"commander":  KillCommander,
	"combo":      KillCombo,
	"mill":       KillMill,
	"poison":     KillPoison,
	"timeout":    KillTimeout,
	"concession": KillConcession,
	"draw":       KillDraw,
}

var killEnumToMethod = [9]string{
	"combat", "commander", "combo", "mill", "poison", "timeout", "unknown",
	"concession", "draw",
}

func killMethodEnum(s string) uint8 {
	if v, ok := killMethodToEnum[s]; ok {
		return v
	}
	return KillUnknown
}

func killMethodString(e uint8) string {
	if int(e) < len(killEnumToMethod) {
		return killEnumToMethod[e]
	}
	return "unknown"
}

// DeckKeyHash returns the FNV-32a hash of a deck key string.
func DeckKeyHash(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32()
}

// DeckIndex maps FNV-32a hashes to full deck key strings for resolving
// binary seeds back to replayable GameSeeds.
type DeckIndex struct {
	mu      sync.RWMutex
	entries map[uint32]string
	path    string
	dirty   bool
}

func NewDeckIndex(path string) *DeckIndex {
	di := &DeckIndex{
		entries: make(map[uint32]string),
		path:    path,
	}
	di.load()
	return di
}

func (di *DeckIndex) load() {
	data, err := os.ReadFile(di.path)
	if err != nil {
		return
	}
	raw := make(map[string]string)
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	for _, v := range raw {
		h := DeckKeyHash(v)
		di.entries[h] = v
	}
}

func (di *DeckIndex) Register(key string) uint32 {
	h := DeckKeyHash(key)
	di.mu.RLock()
	_, exists := di.entries[h]
	di.mu.RUnlock()
	if exists {
		return h
	}
	di.mu.Lock()
	di.entries[h] = key
	di.dirty = true
	di.mu.Unlock()
	return h
}

func (di *DeckIndex) Resolve(hash uint32) string {
	di.mu.RLock()
	defer di.mu.RUnlock()
	return di.entries[hash]
}

func (di *DeckIndex) Flush() {
	di.mu.Lock()
	defer di.mu.Unlock()
	if !di.dirty {
		return
	}
	raw := make(map[string]string, len(di.entries))
	for h, k := range di.entries {
		raw[fmt.Sprintf("%08x", h)] = k
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(di.path, data, 0644); err != nil {
		log.Printf("heimdall: write deck-index %s: %v (dirty flag preserved for retry)", di.path, err)
		return
	}
	di.dirty = false
}

// EncodeSeed packs a GameSeed into exactly 28 bytes.
//
//	[0:8]   RNGSeed     int64  LE
//	[8:12]  DeckHash[0] uint32 LE
//	[12:16] DeckHash[1] uint32 LE
//	[16:20] DeckHash[2] uint32 LE
//	[20:24] DeckHash[3] uint32 LE
//	[24]    Winner      uint8  (0-3, 0xFF = no winner)
//	[25:27] Turns       uint16 LE
//	[27]    KillMethod  uint8  enum
func EncodeSeed(seed GameSeed, di *DeckIndex) [SeedRecordSize]byte {
	var buf [SeedRecordSize]byte
	binary.LittleEndian.PutUint64(buf[0:8], uint64(seed.RNGSeed))
	for i := 0; i < 4; i++ {
		h := di.Register(seed.DeckKeys[i])
		binary.LittleEndian.PutUint32(buf[8+i*4:12+i*4], h)
	}
	if seed.Winner < 0 || seed.Winner > 3 {
		buf[24] = 0xFF
	} else {
		buf[24] = uint8(seed.Winner)
	}
	binary.LittleEndian.PutUint16(buf[25:27], uint16(seed.Turns))
	buf[27] = killMethodEnum(seed.KillMethod)
	return buf
}

// DecodeSeed unpacks 28 bytes into a GameSeed. Deck hashes are resolved
// via the DeckIndex; unresolved hashes produce empty deck key strings.
func DecodeSeed(data [SeedRecordSize]byte, di *DeckIndex) GameSeed {
	s := GameSeed{
		RNGSeed: int64(binary.LittleEndian.Uint64(data[0:8])),
	}
	for i := 0; i < 4; i++ {
		h := binary.LittleEndian.Uint32(data[8+i*4 : 12+i*4])
		s.DeckKeys[i] = di.Resolve(h)
	}
	w := data[24]
	if w == 0xFF {
		s.Winner = -1
	} else {
		s.Winner = int(w)
	}
	s.Turns = int(binary.LittleEndian.Uint16(data[25:27]))
	s.KillMethod = killMethodString(data[27])
	return s
}

// WriteBinarySeeds appends seeds to a binary file. Creates the file with
// a header if it doesn't exist. Returns the file size after writing.
func WriteBinarySeeds(path string, seeds []GameSeed, di *DeckIndex) (int64, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}

	if fi.Size() == 0 {
		var hdr [seedHeaderSize]byte
		binary.LittleEndian.PutUint32(hdr[0:4], seedMagic)
		binary.LittleEndian.PutUint32(hdr[4:8], seedVersion)
		if _, err := f.Write(hdr[:]); err != nil {
			return 0, err
		}
	}

	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return 0, err
	}

	w := bufio.NewWriter(f)
	for _, s := range seeds {
		rec := EncodeSeed(s, di)
		if _, err := w.Write(rec[:]); err != nil {
			return 0, err
		}
	}
	if err := w.Flush(); err != nil {
		return 0, err
	}

	fi, _ = f.Stat()
	return fi.Size(), nil
}

// ReadBinarySeeds reads up to limit seeds from a binary seed file.
// If limit <= 0, all seeds are read.
func ReadBinarySeeds(path string, limit int, di *DeckIndex) ([]GameSeed, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var hdr [seedHeaderSize]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	magic := binary.LittleEndian.Uint32(hdr[0:4])
	if magic != seedMagic {
		return nil, fmt.Errorf("bad magic: %08x (expected %08x)", magic, seedMagic)
	}

	fi, _ := f.Stat()
	dataSize := fi.Size() - seedHeaderSize
	recordCount := int(dataSize / SeedRecordSize)
	if limit > 0 && recordCount > limit {
		recordCount = limit
	}

	seeds := make([]GameSeed, 0, recordCount)
	r := bufio.NewReader(f)
	var buf [SeedRecordSize]byte
	for i := 0; i < recordCount; i++ {
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			break
		}
		seeds = append(seeds, DecodeSeed(buf, di))
	}
	return seeds, nil
}
