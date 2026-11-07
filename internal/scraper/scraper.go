package scraper

import (
	"encoding/json"
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

	"okiscraper/internal/models"
)

type DownloadJob struct {
	VideoDownloadUrl string
	Move             string
	Character        string
}

func ParseTotalMoveCountFromPage(page *rod.Page, tekkenCharacterName string) (int, error) {
	spanPageOf, err := page.Element(".moves_container + div > .text-unselected-grey > span")
	if err != nil {
		return -1, err
	}

	spanPageOfText, err := spanPageOf.Text()
	if err != nil {
		return -1, err
	}

	parts := strings.Split(spanPageOfText, "of ")
	if len(parts) < 2 {
		return -1, fmt.Errorf("could not parse move count from %q", spanPageOfText)
	}

	totalMoves, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return -1, err
	}

	return totalMoves, nil
}

func GetAllMoveDataCardsFromCurrentPage(page *rod.Page, tekkenCharacterName string) (rod.Elements, error) {
	if err := page.WaitStable(time.Second * 3.0); err != nil {
		return nil, err
	}

	dataCards, err := page.Elements("[data-move-command]")
	if err != nil {
		return nil, err
	}

	fmt.Printf("Total %s Moves Found: %d\n\n", tekkenCharacterName, len(dataCards))

	if dataCards.Empty() {
		return nil, fmt.Errorf("%s character does not exist\n", tekkenCharacterName)
	}

	return dataCards, nil
}

func HoverOverDataCard(dataCard *rod.Element, page *rod.Page) error {
	if _, err := dataCard.Eval(`() => this.scrollIntoView({
		block: "center",
		inline: "center",
		behavior: "auto"
	})`); err != nil {
		return err
	}

	if err := dataCard.Hover(); err != nil {
		return err
	}
	if err := page.WaitStable(time.Second * 3.0); err != nil {
		return err
	}

	return nil
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

				parts := strings.SplitAfter(urlString, fmt.Sprintf("%s/", selectedCharacterName))
				if len(parts) < 2 {
					return
				}
				moveName, err := url.QueryUnescape(strings.Split(parts[1], "_")[0])
				if err != nil {
					fmt.Printf("Skipping media %s: %v\n", urlString, err)
					return
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

func GetFrameDataFromDataCardInCompactView(dataCard *rod.Element) (*models.Move, error) {
	mainContainer, err := dataCard.Element(":scope > :nth-child(1)")
	if err != nil {
		return nil, err
	}

	startUpEl, err := mainContainer.Element(":scope > :nth-child(2)")
	if err != nil {
		return nil, err
	}
	startUp, err := startUpEl.Text()
	if err != nil {
		return nil, err
	}
	if strings.Contains(startUp, ",") {
		startUp = strings.Split(startUp, ",")[0]
	}

	hitLevelEl, err := mainContainer.Element(":scope > :nth-child(3)")
	if err != nil {
		return nil, err
	}
	hitLevel, err := hitLevelEl.Text()
	if err != nil {
		return nil, err
	}

	framesOnBlockEl, err := mainContainer.Element(":scope > :nth-child(4) span")
	if err != nil {
		return nil, err
	}
	framesOnBlock, err := framesOnBlockEl.Text()
	if err != nil {
		return nil, err
	}

	framesOnHitEl, err := mainContainer.Element(":scope > :nth-child(5) span")
	if err != nil {
		return nil, err
	}
	framesOnHit, err := framesOnHitEl.Text()
	if err != nil {
		return nil, err
	}

	notes := ""
	if notesEl, err := mainContainer.Element(":scope > :nth-child(8)"); err == nil {
		if t, err := notesEl.Text(); err == nil {
			notes = t
		}
	}

	moveData := models.Move{}

	moveData.Startup = startUp
	moveData.HitLevel = hitLevel
	moveData.FramesOnBlock = framesOnBlock
	moveData.FramesOnHit = framesOnHit

	if notes != "" {
		moveData.Notes = notes
	}

	return &moveData, nil
}

func GetMoveNameFromDataCard(dataCard *rod.Element) (*string, error) {
	return dataCard.Attribute("data-move-command")
}

func SaveMoveFrameDataToJson(frameData []models.Move, filename string, saveDir string) error {
	file, err := os.Create(filepath.Join(saveDir, filename))
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")

	return encoder.Encode(frameData)
}

func ScrapeOkizemeForVideos(browser *rod.Browser, page *rod.Page, selectedCharacterName string, scraperOutputDirName string, totalMoveCount int) error {
	router, downloadJobs, wg := InitNetworkMediaDownloadCallback(browser, selectedCharacterName, scraperOutputDirName)
	go router.Run()

	if err := page.Navigate(
		fmt.Sprintf("https://okizeme.gg/database/%s?movesPerPage=%d", selectedCharacterName, totalMoveCount),
	); err != nil {
		return err
	}
	if err := page.WaitStable(time.Second * 3.0); err != nil {
		return err
	}

	dataCards, err := GetAllMoveDataCardsFromCurrentPage(page, selectedCharacterName)
	if err != nil {
		return err
	}

	for i, dataCard := range dataCards {
		moveName, err := GetMoveNameFromDataCard(dataCard)
		if err != nil {
			return err
		}
		if moveName == nil {
			fmt.Printf("Skipping card %d/%d: no move command attribute\n", i+1, len(dataCards))
			continue
		}

		fmt.Printf("Processing card %d/%d [%s]\n", i+1, len(dataCards), *moveName)
		if err := HoverOverDataCard(dataCard, page); err != nil {
			return err
		}
	}

	router.Stop()
	close(downloadJobs)
	wg.Wait()

	return nil
}

func ScrapeOkizemeForFrameData(
	browser *rod.Browser, page *rod.Page,
	selectedCharacterName string, scraperOutputDirName string, totalMoveCount int,
) error {

	if err := page.Navigate(
		fmt.Sprintf("https://okizeme.gg/database/%s?movesPerPage=%d&view=compact", selectedCharacterName, totalMoveCount),
	); err != nil {
		return err
	}
	if err := page.WaitStable(time.Second * 3.0); err != nil {
		return err
	}

	dataCards, err := GetAllMoveDataCardsFromCurrentPage(page, selectedCharacterName)
	if err != nil {
		return err
	}

	allFrameData := make([]models.Move, 0, totalMoveCount)
	for i, dataCard := range dataCards {
		moveName, err := GetMoveNameFromDataCard(dataCard)
		if err != nil {
			return err
		}
		if moveName == nil {
			fmt.Printf("Skipping card %d/%d: no move command attribute\n", i+1, len(dataCards))
			continue
		}

		fmt.Printf("Processing frame data %d/%d [%s]\n", i+1, len(dataCards), *moveName)

		frameData, err := GetFrameDataFromDataCardInCompactView(dataCard)
		if err != nil {
			return err
		}

		frameData.Command = *moveName
		allFrameData = append(allFrameData, *frameData)
	}

	fmt.Printf("Encoding %s's frame data to JSON...\n\n", selectedCharacterName)

	err = SaveMoveFrameDataToJson(allFrameData, "frame_data.json", filepath.Join(scraperOutputDirName, selectedCharacterName))
	if err != nil {
		return err
	}

	return nil
}

func Scrape(character, outputDir string, downloadVideos, getFrameData bool) error {
	fmt.Printf("\nScraping okizeme.gg...\n\n")

	browser := rod.New()
	if err := browser.Connect(); err != nil {
		return err
	}
	defer browser.Close()

	page, err := stealth.Page(browser)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(outputDir, character), os.ModePerm); err != nil {
		return err
	}

	if err := page.Navigate(fmt.Sprintf("https://okizeme.gg/database/%s", character)); err != nil {
		return err
	}
	if err := page.WaitStable(time.Second * 3.0); err != nil {
		return err
	}

	totalMoves, err := ParseTotalMoveCountFromPage(page, character)
	if err != nil {
		return err
	}

	if downloadVideos {
		if err := ScrapeOkizemeForVideos(browser, page, character, outputDir, totalMoves); err != nil {
			return err
		}
	}

	if getFrameData {
		if err := ScrapeOkizemeForFrameData(browser, page, character, outputDir, totalMoves); err != nil {
			return err
		}
	}

	return nil
}
