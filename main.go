package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
)

type DownloadJob struct {
	URL       string
	Move      string
	Character string
}

func main() {

	var tekkenCharacterName string
	fmt.Print("Enter Tekken 8 Character Name (must be available in okizeme.gg): ")
	fmt.Scan(&tekkenCharacterName)

	tekkenCharacterName = strings.ToLower(tekkenCharacterName)

	startTime := time.Now()

	downloadJobs := make(chan DownloadJob, 24)

	var wg sync.WaitGroup

	for i := 1; i <= 4; i++ {
		wg.Add(1)
		go downloadWorker(i, downloadJobs, &wg)
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

				downloadJobs <- DownloadJob{
					URL:       urlString,
					Move:      moveName,
					Character: tekkenCharacterName,
				}
			}
		}

	})

	go router.Run()

	page.MustNavigate(fmt.Sprintf("https://okizeme.gg/database/%s", tekkenCharacterName)).MustWaitStable()
	spanPageOf := page.MustElement(".moves_container + div > .text-unselected-grey > span")
	totalMoves, err := strconv.Atoi(strings.Split(spanPageOf.MustText(), "of ")[1])
	if err != nil {
		panic(err)
	}

	page.MustNavigate(
		fmt.Sprintf("https://okizeme.gg/database/%s?movesPerPage=%d", tekkenCharacterName, totalMoves),
	).MustWaitStable()

	dataCards := page.MustWaitStable().MustElements("[data-move-command]")

	fmt.Printf("Total Moves Found: %d\n\n", len(dataCards))

	if dataCards.Empty() {
		panic(fmt.Errorf("%s character does not exist\n", tekkenCharacterName))
	}

	err = os.RemoveAll(tekkenCharacterName)
	if err != nil {
		panic(err)
	}

	err = os.Mkdir(tekkenCharacterName, os.ModePerm)
	if err != nil {
		panic(err)
	}

	for i, dataCard := range dataCards {

		fmt.Printf("Processing card %d/%d\n", i+1, len(dataCards))
		dataCard.MustEval(`() => this.scrollIntoView({
			block: "center",
			inline: "center",
			behavior: "auto"
		})`)

		dataCard.MustHover()
		page.MustWaitStable()
	}

	router.Stop()

	close(downloadJobs)

	wg.Wait()

	fmt.Printf("Total Time: %s", time.Since(startTime).String())
}

func downloadWorker(id int, jobs <-chan DownloadJob, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range jobs {
		start := time.Now()
		fmt.Printf("Downloading [%s]: %s\n", job.Move, job.URL)

		if err := downloadVideo(job.URL, job.Move, job.Character); err != nil {
			fmt.Printf(
				"Worker %d failed: %v\n",
				id,
				err,
			)
			continue
		}

		elapsed := time.Since(start)
		fmt.Printf(
			"Finished downloading [%s] in %s\n\n",
			job.Move,
			elapsed.String(),
		)
	}
}

func downloadVideo(url string, move string, characterName string) error {
	client := &http.Client{
		Timeout: time.Second * 15,
	}

	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Unexpected HTTP Status: %s", response.Status)
	}

	filename := filepath.Join(characterName, fmt.Sprintf("%s.mp4", move))

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, response.Body)

	return err
}
