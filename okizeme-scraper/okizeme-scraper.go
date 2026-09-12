package okizemescraper

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
)

type DownloadJob struct {
	VideoDownloadUrl string
	Move             string
	Character        string
}

type MoveFrameData struct {
	MoveName      string `json:"move_name"`
	Startup       string `json:"startup"`
	HitLevel      string `json:"hit_level"`
	FramesOnBlock string `json:"frames_on_block"`
	FramesOnHit   string `json:"frames_on_hit"`
	Notes         string `json:"notes"`
}

func ParseTotalMoveCountFromPage(page *rod.Page, tekkenCharacterName string) (int, error) {
	spanPageOf := page.MustElement(".moves_container + div > .text-unselected-grey > span")
	totalMoves, err := strconv.Atoi(strings.Split(spanPageOf.MustText(), "of ")[1])
	if err != nil {
		return -1, err
	}

	return totalMoves, nil
}

func GetAllMoveDataCardsFromCurrentPage(page *rod.Page, tekkenCharacterName string) (rod.Elements, error) {
	dataCards := page.MustWaitStable().MustElements("[data-move-command]")

	fmt.Printf("Total %s Moves Found: %d\n\n", tekkenCharacterName, len(dataCards))

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

func GetFrameDataFromDataCardInCompactView(dataCard *rod.Element) (*MoveFrameData, error) {
	mainContainer := dataCard.MustElement(":scope > :nth-child(1)")

	startUp := mainContainer.MustElement(":scope > :nth-child(2)").MustText()
	if strings.Contains(startUp, ",") {
		startUp = strings.Split(startUp, ",")[0]
	}

	hitLevel := mainContainer.MustElement(":scope > :nth-child(3)").MustText()
	framesOnBlock := mainContainer.MustElement(":scope > :nth-child(4) span").MustText()
	framesOnHit := mainContainer.MustElement(":scope > :nth-child(5) span").MustText()
	notes, _ := mainContainer.MustElement(":scope > :nth-child(8)").Text()

	moveData := MoveFrameData{}

	moveData.Startup = startUp
	moveData.HitLevel = hitLevel
	moveData.FramesOnBlock = framesOnBlock
	moveData.FramesOnHit = framesOnHit

	if notes != "" {
		moveData.Notes = notes
	}

	return &moveData, nil
}

func GetMoveNameFromDataCard(dataCard *rod.Element) *string {
	return dataCard.MustAttribute("data-move-command")
}

func SaveMoveFrameDataToJson(frameData []MoveFrameData, filename string, saveDir string) error {
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

	page.MustNavigate(
		fmt.Sprintf("https://okizeme.gg/database/%s?movesPerPage=%d", selectedCharacterName, totalMoveCount),
	).MustWaitStable()

	dataCards, err := GetAllMoveDataCardsFromCurrentPage(page, selectedCharacterName)
	if err != nil {
		return err
	}

	for i, dataCard := range dataCards {
		moveName := GetMoveNameFromDataCard(dataCard)

		fmt.Printf("Processing card %d/%d [%s]\n", i+1, len(dataCards), *moveName)
		HoverOverDataCard(dataCard, page)
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

	page.MustNavigate(
		fmt.Sprintf("https://okizeme.gg/database/%s?movesPerPage=%d&view=compact", selectedCharacterName, totalMoveCount),
	).MustWaitStable()

	dataCards, err := GetAllMoveDataCardsFromCurrentPage(page, selectedCharacterName)
	if err != nil {
		return err
	}

	allFrameData := make([]MoveFrameData, 0, totalMoveCount)
	for i, dataCard := range dataCards {
		moveName := GetMoveNameFromDataCard(dataCard)

		fmt.Printf("Processing frame data %d/%d [%s]\n", i+1, len(dataCards), *moveName)

		frameData, err := GetFrameDataFromDataCardInCompactView(dataCard)
		if err != nil {
			return err
		}

		frameData.MoveName = *moveName
		allFrameData = append(allFrameData, *frameData)
	}

	fmt.Printf("Encoding %s's frame data to JSON...\n\n", selectedCharacterName)

	err = SaveMoveFrameDataToJson(allFrameData, "frame_data.json", filepath.Join(scraperOutputDirName, selectedCharacterName))
	if err != nil {
		return err
	}

	return nil
}
