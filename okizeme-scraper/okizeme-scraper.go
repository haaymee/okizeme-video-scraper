package okizemescraper

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
)

type DownloadJob struct {
	VideoDownloadUrl string
	Move             string
	Character        string
}

func ParseTotalMoveCountFromPage(page *rod.Page, tekkenCharacterName string) (int, error) {
	page.MustNavigate(fmt.Sprintf("https://okizeme.gg/database/%s", tekkenCharacterName)).MustWaitStable()
	spanPageOf := page.MustElement(".moves_container + div > .text-unselected-grey > span")
	totalMoves, err := strconv.Atoi(strings.Split(spanPageOf.MustText(), "of ")[1])
	if err != nil {
		return -1, err
	}

	return totalMoves, nil
}

func GetAllMoveDataCardsFromCurrentPage(page *rod.Page, tekkenCharacterName string) (rod.Elements, error) {
	dataCards := page.MustWaitStable().MustElements("[data-move-command]")

	fmt.Printf("Total Moves Found: %d\n\n", len(dataCards))

	if dataCards.Empty() {
		return nil, fmt.Errorf("%s character does not exist\n", tekkenCharacterName)
	}

	return dataCards, nil
}

func HoverOverDataCard(dataCard *rod.Element, page *rod.Page) {
	dataCard.MustEval(`() => this.scrollIntoView({
		block: "center",
		inline: "center",
		behavior: "auto"
	})`)

	dataCard.MustHover()
	page.MustWaitStable()

}

func DownloadWorker(outputDir string, id int, jobs <-chan DownloadJob, wg *sync.WaitGroup, client *http.Client) {
	defer wg.Done()

	for job := range jobs {
		start := time.Now()
		fmt.Printf("Downloading [%s]: %s\n", job.Move, job.VideoDownloadUrl)

		if err := downloadVideo(client, job.VideoDownloadUrl, job.Move, job.Character, outputDir); err != nil {
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

func downloadVideo(client *http.Client, url string, move string, characterName string, outputDir string) error {
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Unexpected HTTP Status: %s", response.Status)
	}

	filename := filepath.Join(outputDir, characterName, fmt.Sprintf("%s.mp4", move))

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, response.Body)

	return err
}

func InitNetworkMediaDownloadCallback(browser *rod.Browser, selectedCharacterName string, scraperOutputDirName string) (
	*rod.HijackRouter,
	chan DownloadJob,
	*sync.WaitGroup,
) {
	router := browser.HijackRequests()
	client := http.Client{
		Timeout: 15 * time.Second,
	}

	downloadJobs := make(chan DownloadJob, 24)

	var wg sync.WaitGroup

	for i := 1; i <= 4; i++ {
		wg.Add(1)
		go DownloadWorker(scraperOutputDirName, i, downloadJobs, &wg, &client)
	}

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

				downloadJobs <- DownloadJob{
					VideoDownloadUrl: urlString,
					Move:             moveName,
					Character:        selectedCharacterName,
				}
			}
		}

	})
	return router, downloadJobs, &wg
}
