package app

import (
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

const scraperOutputDir = "scraper_output"

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

	return a.convertVideosToWebm()
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

	jobs := make(chan convertJob, 10)
	var wg sync.WaitGroup

	for i := 0; i < 4; i++ {
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
