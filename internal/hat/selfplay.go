package hat

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"time"
)

// SelfPlayManager coordinates the Level 6 self-play loop:
//   grinder games → collect samples → train neural model → reload → repeat
//
// The loop runs alongside the grinder. After every TrainThreshold new
// samples, it invokes the Python training script (CUDA or CPU), waits
// for completion, and hot-reloads the model into the evaluator.

type SelfPlayConfig struct {
	SamplesPath    string // data/training/samples.jsonl
	ModelPath      string // data/training/model.json
	CheckpointDir  string // data/training/checkpoints
	TrainScript    string // scripts/train_neural_eval.py
	TrainThreshold int    // samples before triggering training (default 10000)
	Epochs         int    // training epochs per generation (default 200)
	PythonBin      string // "python3" or path to venv python
}

func DefaultSelfPlayConfig(baseDir string) SelfPlayConfig {
	return SelfPlayConfig{
		SamplesPath:    filepath.Join(baseDir, "data/training/samples.jsonl"),
		ModelPath:      filepath.Join(baseDir, "data/training/model.json"),
		CheckpointDir:  filepath.Join(baseDir, "data/training/checkpoints"),
		TrainScript:    filepath.Join(baseDir, "scripts/train_neural_eval.py"),
		TrainThreshold: 10000,
		Epochs:         200,
		PythonBin:      "python3",
	}
}

type SelfPlayManager struct {
	config       SelfPlayConfig
	generation   int32
	sampleCount  int64
	lastTrained  int64
	training     int32 // atomic flag: 1 = training in progress
	lastAttempt  int64 // unix timestamp of last training attempt (success or failure)
	OnModelLoad  func(ne *NeuralEvaluator) // callback to hot-swap model
}

func NewSelfPlayManager(cfg SelfPlayConfig) *SelfPlayManager {
	return &SelfPlayManager{config: cfg}
}

// Generation returns the current model generation number.
func (sp *SelfPlayManager) Generation() int {
	return int(atomic.LoadInt32(&sp.generation))
}

// SampleCount returns total samples collected.
func (sp *SelfPlayManager) SampleCount() int64 {
	return atomic.LoadInt64(&sp.sampleCount)
}

// IsTraining returns true if a training run is currently active.
func (sp *SelfPlayManager) IsTraining() bool {
	return atomic.LoadInt32(&sp.training) == 1
}

// RecordSamples increments the sample counter and triggers training
// if the threshold is met. Non-blocking: training runs in a goroutine.
// Enforces a 5-minute cooldown between attempts to avoid spam on failure.
func (sp *SelfPlayManager) RecordSamples(n int) {
	newCount := atomic.AddInt64(&sp.sampleCount, int64(n))
	lastTrained := atomic.LoadInt64(&sp.lastTrained)
	threshold := int64(sp.config.TrainThreshold)

	if newCount-lastTrained < threshold {
		return
	}
	now := time.Now().Unix()
	lastAttempt := atomic.LoadInt64(&sp.lastAttempt)
	if now-lastAttempt < 1800 {
		return
	}
	if atomic.CompareAndSwapInt32(&sp.training, 0, 1) {
		atomic.StoreInt64(&sp.lastAttempt, now)
		go sp.runTraining(newCount)
	}
}

func (sp *SelfPlayManager) runTraining(sampleCountAtStart int64) {
	defer atomic.StoreInt32(&sp.training, 0)

	gen := atomic.AddInt32(&sp.generation, 1)
	log.Printf("[selfplay] generation %d: training started (%d samples)",
		gen, sampleCountAtStart)

	t0 := time.Now()

	cmd := exec.Command("nice", "-n", "15",
		sp.config.PythonBin, sp.config.TrainScript,
		"--samples", sp.config.SamplesPath,
		"--output", sp.config.ModelPath,
		"--checkpoint-dir", sp.config.CheckpointDir,
		"--epochs", fmt.Sprintf("%d", sp.config.Epochs),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	elapsed := time.Since(t0)

	if err != nil {
		log.Printf("[selfplay] generation %d: training FAILED after %v: %v",
			gen, elapsed, err)
		// Advance lastTrained to avoid immediate re-trigger on failure.
		// Next attempt after another TrainThreshold samples accumulate.
		atomic.StoreInt64(&sp.lastTrained, sampleCountAtStart)
		return
	}

	log.Printf("[selfplay] generation %d: training completed in %v", gen, elapsed)
	atomic.StoreInt64(&sp.lastTrained, sampleCountAtStart)

	// Hot-reload model.
	ne := TryLoadNeuralEvaluator(sp.config.ModelPath)
	if ne == nil {
		log.Printf("[selfplay] generation %d: model load failed", gen)
		return
	}

	log.Printf("[selfplay] generation %d: model loaded (%d layers)",
		gen, len(ne.Layers))

	if sp.OnModelLoad != nil {
		sp.OnModelLoad(ne)
	}
}

const maxSamplesFileSize = 50 * 1024 * 1024 // 50MB — rotate and restart

// WriteEnrichedSamples writes pivot-enriched training samples and
// increments the self-play sample counter. Rotates the file when it
// exceeds 50MB to prevent unbounded growth.
func (sp *SelfPlayManager) WriteEnrichedSamples(path string, samples []PivotEnrichedSample) error {
	if fi, err := os.Stat(path); err == nil && fi.Size() > maxSamplesFileSize {
		rotated := path + fmt.Sprintf(".%s", time.Now().Format("20060102-150405"))
		if rerr := os.Rename(path, rotated); rerr != nil {
			log.Printf("[selfplay] rotate failed (%s → %s): %v — appending to existing file",
				filepath.Base(path), filepath.Base(rotated), rerr)
		} else {
			log.Printf("[selfplay] rotated samples file (%d MB) → %s",
				fi.Size()/(1024*1024), filepath.Base(rotated))
			cleanOldRotations(path, 3)
		}
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, s := range samples {
		if err := enc.Encode(s); err != nil {
			return err
		}
	}
	sp.RecordSamples(len(samples))
	return nil
}

func cleanOldRotations(basePath string, keep int) {
	dir := filepath.Dir(basePath)
	base := filepath.Base(basePath) + "."
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var rotated []string
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) > len(base) && e.Name()[:len(base)] == base {
			rotated = append(rotated, filepath.Join(dir, e.Name()))
		}
	}
	if len(rotated) <= keep {
		return
	}
	for i := 0; i < len(rotated)-keep; i++ {
		os.Remove(rotated[i])
	}
}

// Status returns a human-readable status string for logging/telemetry.
func (sp *SelfPlayManager) Status() string {
	return fmt.Sprintf("gen=%d samples=%d training=%v",
		sp.Generation(), sp.SampleCount(), sp.IsTraining())
}
