package main

import (
	"fmt"
	"net/url"
	okizemescraper "okiscraper/okizeme-scraper"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
)

func main() {
	ScrapeOkizemeSetup()
}

func ScrapeOkizemeSetup() {
	const scraperOutputDirName = "scraper_output"

	var tekkenCharacterName string
	fmt.Print("Enter Tekken 8 Character Name (must be available in okizeme.gg): ")
	fmt.Scan(&tekkenCharacterName)

	tekkenCharacterName = strings.ToLower(tekkenCharacterName)

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

				moveName := strings.SplitAfter(urlString, fmt.Sprintf("%s/", tekkenCharacterName))[1]
				moveName = strings.Split(moveName, "_")[0]
				moveName, err := url.QueryUnescape(moveName)
				if err != nil {
					panic(err)
				}

				downloadJobs <- okizemescraper.DownloadJob{
					VideoDownloadUrl: urlString,
					Move:             moveName,
					Character:        tekkenCharacterName,
					OutputDir:        scraperOutputDirName,
				}
			}
		}

	})

	go router.Run()

	totalMoves, err := okizemescraper.ParseTotalMoveCountFromPage(page, tekkenCharacterName)
	if err != nil {
		panic(err)
	}

	page.MustNavigate(
		fmt.Sprintf("https://okizeme.gg/database/%s?movesPerPage=%d", tekkenCharacterName, totalMoves),
	).MustWaitStable()

	dataCards, err := okizemescraper.GetAllMoveDataCardsFromPage(page, tekkenCharacterName)
	if err != nil {
		panic(err)
	}

	err = os.RemoveAll(filepath.Join(scraperOutputDirName, tekkenCharacterName))
	if err != nil {
		panic(err)
	}

	err = os.MkdirAll(filepath.Join(scraperOutputDirName, tekkenCharacterName), os.ModePerm)
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
