package main

import (
	"fmt"
	ankiintegration "okiscraper/anki-integration"
	okizemescraper "okiscraper/okizeme-scraper"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/huh"
	"github.com/go-rod/rod"
	"github.com/go-rod/stealth"
)

type AppOptions int

const (
	DownloadVideos AppOptions = iota
	GetMoveFrameData
	CreateAnkiDeck
)

func (ao AppOptions) String() string {
	switch ao {
	case DownloadVideos:
		return "Download Videos"
	case GetMoveFrameData:
		return "Get Move Frame Data"
	case CreateAnkiDeck:
		return "Create Anki Deck"
	default:
		return "ERROR"
	}
}

func main() {
	const scraperOutputDirName = "scraper_output"

	selectedCharacterName, selectedAppActions, err := ConfigureApp()
	if err != nil {
		panic(err)
	}

	fmt.Printf("You selected:\n\n")
	for _, action := range selectedAppActions {
		fmt.Printf("\t[%s]\n", action)
	}

	fmt.Print("\nProcessing actions...\n\n")

	shouldScrapeOkizeme := slices.Contains(selectedAppActions, DownloadVideos) || slices.Contains(selectedAppActions, GetMoveFrameData)
	startTime := time.Now()

	if shouldScrapeOkizeme {
		err := ScrapeOkizeme(selectedCharacterName, scraperOutputDirName, selectedAppActions)
		if err != nil {
			panic(err)
		}
	}

	shouldCreateAnkiDeck := slices.Contains(selectedAppActions, CreateAnkiDeck)
	if shouldCreateAnkiDeck {

		_, err := ankiintegration.CreateDeck(fmt.Sprintf("Tekken 8::%s Moves", capitalizeFirst(selectedCharacterName)))
		if err != nil {
			panic(err)
		}

		response, err := ankiintegration.GetNoteTypes()
		if err != nil {
			panic(err)
		}

		okiscraperNoteTypeExists := false
		for _, note := range response {
			if note == ankiintegration.OKI_SCRAPER_NOTE_TYPE_NAME {
				okiscraperNoteTypeExists = true
				break
			}
		}

		if !okiscraperNoteTypeExists {
			fmt.Printf("Okiscraper Note Type does not exist. Creating note type...\n\n")
			err = ankiintegration.CreateNoteType(ankiintegration.OKI_SCRAPER_NOTE_TYPE_NAME)
			if err != nil {
				panic(err)
			}
		} else {
			fmt.Printf("Okiscraper Note Type already exists\n\n")
		}

		webmOutputDir := filepath.Join(scraperOutputDirName, selectedCharacterName, "webm")
		os.Remove(webmOutputDir)
		os.MkdirAll(webmOutputDir, os.ModePerm)

		files, err := filepath.Glob(filepath.Join(scraperOutputDirName, selectedCharacterName, "*.mp4"))
		if err != nil {
			panic(err)
		}

		conversionJobs := make(chan ankiintegration.MP4ToWebmConversionJob, 10)
		var wg sync.WaitGroup

		for i := 0; i < 4; i++ {
			wg.Add(1)

			go func(workerID int) {
				defer wg.Done()

				for job := range conversionJobs {
					fmt.Printf("Worker %d converting %s\n", workerID, filepath.Base(job.InputPath))

					conversionErr := ankiintegration.ConvertMp4ToWebm(job.InputPath, job.OutputPath)
					if conversionErr != nil {
						fmt.Printf("Conversion failed: %v\n", conversionErr)
						continue
					}

					fmt.Printf("Converted %s\n",
						filepath.Base(job.OutputPath),
					)
				}
			}(i)
		}

		for _, file := range files {

			fmt.Printf("Processing %s\n", file)

			outputFileName := selectedCharacterName + "_" + strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)) + ".webm"
			outputFilePath := filepath.Join(webmOutputDir, outputFileName)

			conversionJobs <- ankiintegration.MP4ToWebmConversionJob{
				InputPath:  file,
				OutputPath: outputFilePath,
			}
		}

		close(conversionJobs)
		wg.Wait()

	}

	fmt.Printf("Total Time: %s", time.Since(startTime).String())
}

func ScrapeOkizeme(selectedCharacterName string, scraperOutputDirName string, selectedAppActions []AppOptions) error {

	fmt.Printf("\nScraping okizeme.gg...\n\n")

	browser := rod.New().MustConnect()
	defer browser.MustClose()

	page := stealth.MustPage(browser)

	err := InitializeOutputDirectory(scraperOutputDirName, selectedCharacterName)
	if err != nil {
		return err
	}

	shouldDownloadVideos := slices.Contains(selectedAppActions, DownloadVideos)
	shouldGetFrameData := slices.Contains(selectedAppActions, GetMoveFrameData)

	page.MustNavigate(fmt.Sprintf("https://okizeme.gg/database/%s", selectedCharacterName)).MustWaitStable()
	totalMoves, err := okizemescraper.ParseTotalMoveCountFromPage(page, selectedCharacterName)
	if err != nil {
		return err
	}

	if shouldDownloadVideos {
		err = okizemescraper.ScrapeOkizemeForVideos(browser, page, selectedCharacterName, scraperOutputDirName, totalMoves)
		if err != nil {
			return err
		}
	}

	if shouldGetFrameData {
		err = okizemescraper.ScrapeOkizemeForFrameData(browser, page, selectedCharacterName, scraperOutputDirName, totalMoves)
		if err != nil {
			return err
		}
	}

	return nil
}

func InitializeOutputDirectory(scraperOutputDirName string, selectedCharacterName string) error {
	// err := os.RemoveAll(filepath.Join(scraperOutputDirName, selectedCharacterName))
	// if err != nil {
	// 	return err
	// }

	err := os.MkdirAll(filepath.Join(scraperOutputDirName, selectedCharacterName), os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

func ConfigureApp() (string, []AppOptions, error) {
	var tekkenCharacterName string
	fmt.Print("Enter Tekken 8 Character Name (must be available in okizeme.gg): ")
	fmt.Scan(&tekkenCharacterName)

	tekkenCharacterName = strings.ToLower(tekkenCharacterName)

	var selectedOptions []AppOptions

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[AppOptions]().
				Title("What would you like to do?").
				Description("Use Space to select/deselect, Enter to confirm.").
				Options(
					huh.NewOption(fmt.Sprintf("Download all move videos for %s", tekkenCharacterName), DownloadVideos),
					huh.NewOption(fmt.Sprintf("Get frame data for all %s's moves. (For Anki)", tekkenCharacterName), GetMoveFrameData),
					huh.NewOption("Create Anki Deck (requires selecting 1st and 2nd option)", CreateAnkiDeck),
				).
				Value(&selectedOptions),
		),
	)

	err := form.Run()
	if err != nil {
		return "", nil, err
	}

	// if slices.Contains(selectedOptions, CreateAnkiDeck) {
	// 	if !slices.Contains(selectedOptions, DownloadVideos) || !slices.Contains(selectedOptions, GetMoveFrameData) {
	// 		selectedOptions = slices.DeleteFunc(selectedOptions, func(o AppOptions) bool {
	// 			return o == CreateAnkiDeck
	// 		})
	// 	}
	// }

	return tekkenCharacterName, selectedOptions, nil
}

func capitalizeFirst(s string) string {
	if s == "" {
		return ""
	}
	// Decode the first rune (character) and get its byte size
	r, size := utf8.DecodeRuneInString(s)

	// Convert the single rune to uppercase and append the rest of the string
	return string(unicode.ToUpper(r)) + s[size:]
}
