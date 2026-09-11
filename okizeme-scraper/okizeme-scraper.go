package okizemescraper

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
)

type DownloadJob struct {
	VideoDownloadUrl string
	Move             string
	Character        string
	OutputDir        string
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
		panic(fmt.Errorf("%s character does not exist\n", tekkenCharacterName))
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

func DownloadWorker(id int, jobs <-chan DownloadJob, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range jobs {
		start := time.Now()
		fmt.Printf("Downloading [%s]: %s\n", job.Move, job.VideoDownloadUrl)

		if err := downloadVideo(job.VideoDownloadUrl, job.Move, job.Character, job.OutputDir); err != nil {
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

func downloadVideo(url string, move string, characterName string, outputDir string) error {
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

	filename := filepath.Join(outputDir, characterName, fmt.Sprintf("%s.mp4", move))

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, response.Body)

	return err
}
