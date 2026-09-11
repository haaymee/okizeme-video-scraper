package main

import (
	"fmt"
	"net/url"
	okizemescraper "okiscraper/okizeme-scraper"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
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
	ScrapeOkizemeSetup()
}

func ScrapeOkizemeSetup() {
	const scraperOutputDirName = "scraper_output"

	selectedCharacterName, _, err := ConfigureApp()

	fmt.Print("Processing actions...\n\n")

	startTime := time.Now()

	downloadJobs := make(chan okizemescraper.DownloadJob, 24)

	var wg sync.WaitGroup

	for i := 1; i <= 4; i++ {
		wg.Add(1)
		go okizemescraper.DownloadWorker(i, downloadJobs, &wg)
	}

	browser := rod.New().MustConnect()
	defer browser.MustClose()

	page := stealth.MustPage(browser)

	router := browser.HijackRequests()

	cachedVideos := make(map[string]bool)
	router.Add("*", proto.NetworkResourceTypeMedia, func(ctx *rod.Hijack) {
		urlString := ctx.Request.URL().String()

		ctx.ContinueRequest(&proto.FetchContinueRequest{})

		if strings.Contains(urlString, ".mp4") {

			if _, keyExists := cachedVideos[urlString]; !keyExists {
				cachedVideos[urlString] = true

				moveName := strings.SplitAfter(urlString, fmt.Sprintf("%s/", selectedCharacterName))[1]
				moveName = strings.Split(moveName, "_")[0]
				moveName, err := url.QueryUnescape(moveName)
				if err != nil {
					panic(err)
				}

				downloadJobs <- okizemescraper.DownloadJob{
					VideoDownloadUrl: urlString,
					Move:             moveName,
					Character:        selectedCharacterName,
					OutputDir:        scraperOutputDirName,
				}
			}
		}

	})

	go router.Run()

	totalMoves, err := okizemescraper.ParseTotalMoveCountFromPage(page, selectedCharacterName)
	if err != nil {
		panic(err)
	}

	page.MustNavigate(
		fmt.Sprintf("https://okizeme.gg/database/%s?movesPerPage=%d", selectedCharacterName, totalMoves),
	).MustWaitStable()

	dataCards, err := okizemescraper.GetAllMoveDataCardsFromCurrentPage(page, selectedCharacterName)
	if err != nil {
		panic(err)
	}

	err = InitializeOutputDirectory(scraperOutputDirName, selectedCharacterName)
	if err != nil {
		panic(err)
	}

	for i, dataCard := range dataCards {
		fmt.Printf("Processing card %d/%d\n", i+1, len(dataCards))
		okizemescraper.HoverOverDataCard(dataCard, page)
	}

	router.Stop()

	close(downloadJobs)

	wg.Wait()

	fmt.Printf("Total Time: %s", time.Since(startTime).String())
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

func ConfigureApp() (string, []string, error) {
	var tekkenCharacterName string
	fmt.Print("Enter Tekken 8 Character Name (must be available in okizeme.gg): ")
	fmt.Scan(&tekkenCharacterName)

	tekkenCharacterName = strings.ToLower(tekkenCharacterName)

	var selectedOptions []string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("What would you like to do?").
				Description("Use Space to select/deselect, Enter to confirm.").
				Options(
					huh.NewOption(fmt.Sprintf("Download all move videos for %s", tekkenCharacterName), DownloadVideos.String()),
					huh.NewOption(fmt.Sprintf("Get frame data for all %s's moves. (For Anki)", tekkenCharacterName), GetMoveFrameData.String()),
					huh.NewOption("Create Anki Deck (requires selecting 1st and 2nd option)", CreateAnkiDeck.String()),
				).
				Value(&selectedOptions),
		),
	)

	err := form.Run()
	if err != nil {
		return "", nil, err
	}

	if slices.Contains(selectedOptions, CreateAnkiDeck.String()) {
		if !slices.Contains(selectedOptions, DownloadVideos.String()) || !slices.Contains(selectedOptions, GetMoveFrameData.String()) {
			selectedOptions = slices.DeleteFunc(selectedOptions, func(f string) bool {
				return f == CreateAnkiDeck.String()
			})
		}
	}

	fmt.Printf("You selected %v\n\n", selectedOptions)

	return tekkenCharacterName, selectedOptions, nil
}
