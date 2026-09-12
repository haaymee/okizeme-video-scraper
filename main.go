package main

import (
	"fmt"
	okizemescraper "okiscraper/okizeme-scraper"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

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
		err := ScrapeOkizemeSetup(selectedCharacterName, scraperOutputDirName, selectedAppActions)
		if err != nil {
			panic(err)
		}
	}

	fmt.Printf("Total Time: %s", time.Since(startTime).String())
}

func ScrapeOkizemeSetup(selectedCharacterName string, scraperOutputDirName string, selectedAppActions []AppOptions) error {

	fmt.Printf("\nScraping okizeme.gg...\n\n")

	browser := rod.New().MustConnect()
	defer browser.MustClose()

	page := stealth.MustPage(browser)

	err := InitializeOutputDirectory(scraperOutputDirName, selectedCharacterName)
	if err != nil {
		return err
	}

	var router *rod.HijackRouter
	var downloadJobs chan okizemescraper.DownloadJob
	var wg *sync.WaitGroup
	shouldDownloadVideos := slices.Contains(selectedAppActions, DownloadVideos)
	if shouldDownloadVideos {
		router, downloadJobs, wg = okizemescraper.InitNetworkMediaDownloadCallback(browser, selectedCharacterName, scraperOutputDirName)
		go router.Run()
	}

	totalMoves, err := okizemescraper.ParseTotalMoveCountFromPage(page, selectedCharacterName)
	if err != nil {
		return err
	}

	page.MustNavigate(
		fmt.Sprintf("https://okizeme.gg/database/%s?movesPerPage=%d", selectedCharacterName, totalMoves),
	).MustWaitStable()

	dataCards, err := okizemescraper.GetAllMoveDataCardsFromCurrentPage(page, selectedCharacterName)
	if err != nil {
		return err
	}

	for i, dataCard := range dataCards {
		fmt.Printf("Processing card %d/%d\n", i+1, len(dataCards))
		okizemescraper.HoverOverDataCard(dataCard, page)
	}

	if shouldDownloadVideos {
		router.Stop()
		close(downloadJobs)
		wg.Wait()
	}

	return nil
}

func InitializeOutputDirectory(scraperOutputDirName string, selectedCharacterName string) error {
	err := os.RemoveAll(filepath.Join(scraperOutputDirName, selectedCharacterName))
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Join(scraperOutputDirName, selectedCharacterName), os.ModePerm)
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

	if slices.Contains(selectedOptions, CreateAnkiDeck) {
		if !slices.Contains(selectedOptions, DownloadVideos) || !slices.Contains(selectedOptions, GetMoveFrameData) {
			selectedOptions = slices.DeleteFunc(selectedOptions, func(o AppOptions) bool {
				return o == CreateAnkiDeck
			})
		}
	}

	return tekkenCharacterName, selectedOptions, nil
}
