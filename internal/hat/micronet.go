package hat

import (
	"encoding/json"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hexdek/hexdek/internal/gameengine"
)

const (
	MicroInputDim  = 30
	MicroHidden1   = 64
	MicroHidden2   = 48
	MicroHidden3   = 16
	MicroOutputDim = 1

	microTrainThreshold = 200
	microDefaultLR      = 0.001
	microDefaultEpochs  = 80
	microMomentum       = 0.9
	microMaxSamples     = 5000
)

type MicroLayer struct {
	Weights [][]float64 `json:"weights"`
	Biases  []float64   `json:"biases"`
}

type MicroNet struct {
	Layers     []MicroLayer `json:"layers"`
	DeckKey    string       `json:"deck_key"`
	Generation int          `json:"generation"`
	TrainLoss  float64      `json:"train_loss"`
}

type MicroSample struct {
	Input     [MicroInputDim]float64 `json:"input"`
	Placement float64                `json:"placement"`
}

type MicroCollector struct {
	mu      sync.Mutex
	samples []MicroSample
	deckKey string
}

func NewMicroNet(deckKey string, rng *rand.Rand) *MicroNet {
	mn := &MicroNet{
		DeckKey: deckKey,
		Layers: []MicroLayer{
			randomLayer(MicroInputDim, MicroHidden1, rng),
			randomLayer(MicroHidden1, MicroHidden2, rng),
			randomLayer(MicroHidden2, MicroHidden3, rng),
			randomLayer(MicroHidden3, MicroOutputDim, rng),
		},
	}
	return mn
}

func randomLayer(inDim, outDim int, rng *rand.Rand) MicroLayer {
	scale := math.Sqrt(2.0 / float64(inDim))
	weights := make([][]float64, outDim)
	biases := make([]float64, outDim)
	for i := 0; i < outDim; i++ {
		weights[i] = make([]float64, inDim)
		for j := 0; j < inDim; j++ {
			weights[i][j] = rng.NormFloat64() * scale
		}
	}
	return MicroLayer{Weights: weights, Biases: biases}
}

func (mn *MicroNet) Forward(input [MicroInputDim]float64) float64 {
	current := input[:]
	for i, layer := range mn.Layers {
		outDim := len(layer.Biases)
		next := make([]float64, outDim)
		for j := 0; j < outDim; j++ {
			sum := layer.Biases[j]
			for k, w := range layer.Weights[j] {
				if k < len(current) {
					sum += w * current[k]
				}
			}
			if i < len(mn.Layers)-1 {
				next[j] = relu(sum)
			} else {
				next[j] = sum
			}
		}
		current = next
	}
	if len(current) > 0 {
		return sigmoid(current[0])
	}
	return 0.5
}

func (mn *MicroNet) Train(samples []MicroSample, epochs int, lr float64) float64 {
	if len(samples) == 0 {
		return 0
	}

	nLayers := len(mn.Layers)
	velocity := make([]MicroLayer, nLayers)
	for i, layer := range mn.Layers {
		outDim := len(layer.Biases)
		inDim := len(layer.Weights[0])
		velocity[i].Weights = make([][]float64, outDim)
		velocity[i].Biases = make([]float64, outDim)
		for j := 0; j < outDim; j++ {
			velocity[i].Weights[j] = make([]float64, inDim)
		}
	}

	var finalLoss float64

	for epoch := 0; epoch < epochs; epoch++ {
		grads := make([]MicroLayer, nLayers)
		for i, layer := range mn.Layers {
			outDim := len(layer.Biases)
			inDim := len(layer.Weights[0])
			grads[i].Weights = make([][]float64, outDim)
			grads[i].Biases = make([]float64, outDim)
			for j := 0; j < outDim; j++ {
				grads[i].Weights[j] = make([]float64, inDim)
			}
		}

		totalLoss := 0.0
		batchSize := float64(len(samples))

		for _, s := range samples {
			activations := make([][]float64, nLayers+1)
			activations[0] = s.Input[:]

			for i, layer := range mn.Layers {
				outDim := len(layer.Biases)
				out := make([]float64, outDim)
				for j := 0; j < outDim; j++ {
					sum := layer.Biases[j]
					for k, w := range layer.Weights[j] {
						if k < len(activations[i]) {
							sum += w * activations[i][k]
						}
					}
					if i < nLayers-1 {
						out[j] = relu(sum)
					} else {
						out[j] = sum
					}
				}
				activations[i+1] = out
			}

			pred := sigmoid(activations[nLayers][0])
			err := pred - s.Placement
			totalLoss += err * err

			dOutput := err * pred * (1 - pred)

			deltas := make([][]float64, nLayers)
			deltas[nLayers-1] = []float64{dOutput}

			for i := nLayers - 2; i >= 0; i-- {
				outDim := len(mn.Layers[i].Biases)
				d := make([]float64, outDim)
				for j := 0; j < outDim; j++ {
					var sum float64
					for k := 0; k < len(deltas[i+1]); k++ {
						sum += deltas[i+1][k] * mn.Layers[i+1].Weights[k][j]
					}
					if activations[i+1][j] > 0 {
						d[j] = sum
					}
				}
				deltas[i] = d
			}

			for i := 0; i < nLayers; i++ {
				for j := 0; j < len(mn.Layers[i].Biases); j++ {
					grads[i].Biases[j] += deltas[i][j]
					for k := 0; k < len(mn.Layers[i].Weights[j]); k++ {
						if k < len(activations[i]) {
							grads[i].Weights[j][k] += deltas[i][j] * activations[i][k]
						}
					}
				}
			}
		}

		for i := 0; i < nLayers; i++ {
			for j := 0; j < len(mn.Layers[i].Biases); j++ {
				velocity[i].Biases[j] = microMomentum*velocity[i].Biases[j] - lr*grads[i].Biases[j]/batchSize
				mn.Layers[i].Biases[j] += velocity[i].Biases[j]
				for k := 0; k < len(mn.Layers[i].Weights[j]); k++ {
					velocity[i].Weights[j][k] = microMomentum*velocity[i].Weights[j][k] - lr*grads[i].Weights[j][k]/batchSize
					mn.Layers[i].Weights[j][k] += velocity[i].Weights[j][k]
				}
			}
		}

		finalLoss = totalLoss / batchSize
	}

	mn.TrainLoss = finalLoss
	mn.Generation++
	return finalLoss
}

func EncodeMicroInput(gs *gameengine.GameState, seatIdx int, evalResult EvalResult, plan GamePlan) [MicroInputDim]float64 {
	var input [MicroInputDim]float64

	dims := evalResult.AsArray()
	for i := 0; i < NumDimensions && i < 20; i++ {
		input[i] = dims[i]
	}

	seat := gs.Seats[seatIdx]
	turnNorm := float64(gs.Turn) / 50.0
	if turnNorm > 1.0 {
		turnNorm = 1.0
	}

	var stage float64
	switch {
	case gs.Turn <= 5:
		stage = 0.0
	case gs.Turn <= 12:
		stage = 0.5
	default:
		stage = 1.0
	}

	alive := 0
	avgEval := 0.0
	for _, s := range gs.Seats {
		if !s.Lost && !s.LeftGame {
			alive++
		}
	}

	if alive > 1 {
		for i, s := range gs.Seats {
			if !s.Lost && !s.LeftGame && i != seatIdx {
				avgEval += evalResult.Score
			}
		}
		avgEval /= float64(alive - 1)
	}

	positionVsField := evalResult.Score - avgEval
	if positionVsField > 1.0 {
		positionVsField = 1.0
	}
	if positionVsField < -1.0 {
		positionVsField = -1.0
	}

	input[20] = stage
	input[21] = float64(plan) / 5.0
	input[22] = evalResult.ComboProximity
	input[23] = float64(seat.Life) / 40.0
	input[24] = positionVsField
	input[25] = float64(seat.ManaPool) / 15.0
	input[26] = float64(len(seat.Hand)) / 10.0
	input[27] = turnNorm
	input[28] = float64(alive) / 4.0
	input[29] = float64(len(seat.Library)) / 99.0

	return input
}

func NewMicroCollector(deckKey string) *MicroCollector {
	return &MicroCollector{
		deckKey: deckKey,
		samples: make([]MicroSample, 0, 256),
	}
}

func (mc *MicroCollector) Record(input [MicroInputDim]float64) {
	mc.mu.Lock()
	mc.samples = append(mc.samples, MicroSample{Input: input})
	mc.mu.Unlock()
}

func (mc *MicroCollector) Finalize(placement float64) []MicroSample {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	out := make([]MicroSample, len(mc.samples))
	for i := range mc.samples {
		out[i] = mc.samples[i]
		out[i].Placement = placement
	}
	mc.samples = mc.samples[:0]
	return out
}

func (mc *MicroCollector) Len() int {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	return len(mc.samples)
}

type MicroTrainer struct {
	mu       sync.Mutex
	buffers  map[string][]MicroSample
	nets     map[string]*MicroNet
	dir      string
	rng      *rand.Rand
	training int32
}

func NewMicroTrainer(dir string, rng *rand.Rand) *MicroTrainer {
	os.MkdirAll(dir, 0755)
	return &MicroTrainer{
		buffers: make(map[string][]MicroSample),
		nets:    make(map[string]*MicroNet),
		dir:     dir,
		rng:     rng,
	}
}

func (mt *MicroTrainer) GetNet(deckKey string) *MicroNet {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	return mt.nets[deckKey]
}

func (mt *MicroTrainer) AddSamples(deckKey string, samples []MicroSample) {
	if len(samples) == 0 {
		return
	}
	mt.mu.Lock()
	buf := mt.buffers[deckKey]
	buf = append(buf, samples...)
	if len(buf) > microMaxSamples {
		buf = buf[len(buf)-microMaxSamples:]
	}
	mt.buffers[deckKey] = buf

	shouldTrain := len(buf) >= microTrainThreshold && len(buf)%microTrainThreshold < len(samples)
	mt.mu.Unlock()

	if shouldTrain {
		mt.trainDeck(deckKey)
	}
}

func (mt *MicroTrainer) trainDeck(deckKey string) {
	mt.mu.Lock()
	samples := make([]MicroSample, len(mt.buffers[deckKey]))
	copy(samples, mt.buffers[deckKey])

	net := mt.nets[deckKey]
	if net == nil {
		net = NewMicroNet(deckKey, mt.rng)
		mt.nets[deckKey] = net
	}
	mt.mu.Unlock()

	net.Train(samples, microDefaultEpochs, microDefaultLR)

	mt.mu.Lock()
	mt.nets[deckKey] = net
	mt.mu.Unlock()

	if err := SaveMicroNet(mt.dir, net); err != nil {
		log.Printf("micronet: save %s failed: %v (in-memory net still active)", deckKey, err)
	}
}

func (mt *MicroTrainer) LoadAll() {
	entries, err := os.ReadDir(mt.dir)
	if err != nil {
		return
	}
	mt.mu.Lock()
	defer mt.mu.Unlock()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(mt.dir, e.Name()))
		if err != nil {
			continue
		}
		var net MicroNet
		if err := json.Unmarshal(data, &net); err != nil {
			continue
		}
		if net.DeckKey != "" {
			mt.nets[net.DeckKey] = &net
		}
	}
}

func (mt *MicroTrainer) NetCount() int {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	return len(mt.nets)
}

func SaveMicroNet(dir string, net *MicroNet) error {
	os.MkdirAll(dir, 0755)
	fname := filepath.Join(dir, sanitizeKey(net.DeckKey)+".json")
	data, err := json.Marshal(net)
	if err != nil {
		return err
	}
	return os.WriteFile(fname, data, 0644)
}

func LoadMicroNet(dir, deckKey string) (*MicroNet, error) {
	fname := filepath.Join(dir, sanitizeKey(deckKey)+".json")
	data, err := os.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	var net MicroNet
	if err := json.Unmarshal(data, &net); err != nil {
		return nil, err
	}
	return &net, nil
}

func MicroParamCount(mn *MicroNet) int {
	count := 0
	for _, layer := range mn.Layers {
		for _, w := range layer.Weights {
			count += len(w)
		}
		count += len(layer.Biases)
	}
	return count
}
