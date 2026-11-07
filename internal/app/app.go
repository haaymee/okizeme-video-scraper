package app

import (
	"encoding/json"
	"fmt"
	"okiscraper/internal/anki"
	"okiscraper/internal/cli"
	"okiscraper/internal/media"
	"okiscraper/internal/models"
	"okiscraper/internal/scraper"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

const (
	scraperOutputDir   = "scraper_output"
	convertWorkerCount = 4
	convertJobBuffer   = 10
)

type App struct {
	Character string
	Actions   []cli.AppOptions
	// injected collaborators live here in Stage 2; in Stage 1 keep it minimal
}

func New(character string, actions []cli.AppOptions) *App {
	return &App{Character: character, Actions: actions}
}

func (a *App) Run() error {
	fmt.Printf("You selected:\n\n")
	for _, action := range a.Actions {
		fmt.Printf("\t[%s]\n", action)
	}
	fmt.Print("\nProcessing actions...\n\n")

	if slices.Contains(a.Actions, cli.DownloadVideos) || slices.Contains(a.Actions, cli.GetMoveFrameData) {
		if err := scraper.Scrape(
			a.Character,
			scraperOutputDir,
			slices.Contains(a.Actions, cli.DownloadVideos),
			slices.Contains(a.Actions, cli.GetMoveFrameData),
		); err != nil {
			return err
		}
	}

	if slices.Contains(a.Actions, cli.CreateAnkiDeck) {
		if err := a.setupAnki(); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) setupAnki() error {
	deckName := fmt.Sprintf("Tekken 8::%s Moves", models.Capitalize(a.Character))

	if _, err := anki.CreateDeck(deckName); err != nil {
		return err
	}

	noteTypes, err := anki.GetNoteTypes()
	if err != nil {
		return err
	}

	if !slices.Contains(noteTypes, anki.OKI_SCRAPER_NOTE_TYPE_NAME) {
		fmt.Printf("Okiscraper Note Type does not exist. Creating note type...\n\n")
		if err := anki.CreateNoteType(anki.OKI_SCRAPER_NOTE_TYPE_NAME); err != nil {
			return err
		}
	} else {
		fmt.Printf("Okiscraper Note Type already exists\n\n")
	}

	if err := a.convertVideosToWebm(); err != nil {
		return err
	}

	return a.buildCards(deckName)
}

type convertJob struct {
	InputPath  string
	OutputPath string
}

func (a *App) convertVideosToWebm() error {
	webmOutputDir := filepath.Join(scraperOutputDir, a.Character, "webm")
	if err := os.RemoveAll(webmOutputDir); err != nil {
		return err
	}
	if err := os.MkdirAll(webmOutputDir, os.ModePerm); err != nil {
		return err
	}

	files, err := filepath.Glob(filepath.Join(scraperOutputDir, a.Character, "*.mp4"))
	if err != nil {
		return err
	}

	jobs := make(chan convertJob, convertJobBuffer)
	var wg sync.WaitGroup

	for i := 0; i < convertWorkerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobs {
				fmt.Printf("Worker %d converting %s\n", workerID, filepath.Base(job.InputPath))
				if err := media.ConvertMp4ToWebm(job.InputPath, job.OutputPath); err != nil {
					fmt.Printf("Conversion failed: %v\n", err)
					continue
				}
				fmt.Printf("Converted %s\n", filepath.Base(job.OutputPath))
			}
		}(i)
	}

	for _, file := range files {
		fmt.Printf("Processing %s\n", file)
		outputFileName := a.Character + "_" + strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)) + ".webm"
		jobs <- convertJob{
			InputPath:  file,
			OutputPath: filepath.Join(webmOutputDir, outputFileName),
		}
	}

	close(jobs)
	wg.Wait()

	return nil
}

// buildCards reads the scraped frame data and joins it with the converted .webm
// files (by move Key) to create one Anki note per move. Moves without a video
// are skipped.
func (a *App) buildCards(deckName string) error {
	baseDir := filepath.Join(scraperOutputDir, a.Character)

	moves, err := readFrameData(filepath.Join(baseDir, "frame_data.json"))
	if err != nil {
		return err
	}

	webmByKey, err := webmPathsByKey(filepath.Join(baseDir, "webm"), a.Character)
	if err != nil {
		return err
	}

	ready, skipped := planCards(moves, webmByKey)
	for _, m := range skipped {
		fmt.Printf("Skipping card [%s]: no webm video\n", m.Command)
	}

	created := 0
	for _, c := range ready {
		if err := anki.CreateCard(deckName, a.Character, c.Move, c.VideoPath); err != nil {
			fmt.Printf("Failed to create card [%s]: %v\n", c.Move.Command, err)
			continue
		}
		created++
	}

	fmt.Printf("\nCreated %d cards (%d skipped, no video)\n", created, len(skipped))
	return nil
}

func readFrameData(path string) ([]models.Move, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read frame data (run the frame-data scrape first): %w", err)
	}
	var moves []models.Move
	if err := json.Unmarshal(b, &moves); err != nil {
		return nil, err
	}
	return moves, nil
}

// webmPathsByKey maps each move Key to its absolute .webm path. Paths are made
// absolute because AnkiConnect resolves `path` relative to its own working
// directory (the Anki process), not this program's. Files are named
// "{character}_{key}.webm" (see convertVideosToWebm).
func webmPathsByKey(webmDir, character string) (map[string]string, error) {
	files, err := filepath.Glob(filepath.Join(webmDir, "*.webm"))
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]string, len(files))
	prefix := character + "_"
	for _, f := range files {
		abs, err := filepath.Abs(f)
		if err != nil {
			return nil, err
		}
		base := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
		byKey[strings.TrimPrefix(base, prefix)] = abs
	}
	return byKey, nil
}

type cardPlan struct {
	Move      models.Move
	VideoPath string
}

// planCards pairs each move with its webm by Key; moves without a video are
// returned as skipped. Pure function for unit testing.
func planCards(moves []models.Move, webmByKey map[string]string) (ready []cardPlan, skipped []models.Move) {
	for _, m := range moves {
		if path, ok := webmByKey[m.Key]; ok {
			ready = append(ready, cardPlan{Move: m, VideoPath: path})
		} else {
			skipped = append(skipped, m)
		}
	}
	return ready, skipped
}
