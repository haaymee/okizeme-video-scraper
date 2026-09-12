package ankiintegration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
)

const (
	OKI_SCRAPER_NOTE_TYPE_NAME = "okiscraper-tekken8"

	okiScraperFrontCardTemplate = `
	<video autoplay loop>
	<source src="{{Video}}" type="video/webm">
	</video>
	<br>
	{{Move Command}}
	`

	okiScraperBackCardTemplate = `
	{{FrontSide}}

	<hr id=answer>

	<div class="answer">
		<b>On Block:</b> {{Frames On Block}} <br><br>
		<b>On Hit:</b> {{Frames On Hit}} <br><br>
		<b>Hit Level:</b> {{Hit Level}} <br><br>
		<b>Notes:</b> {{Notes}} <br><br>
	</div>
	`
	okiScraperCardStylingTemplate = `
	.card {
		font-family: arial;
		font-size: 20px;
		line-height: 1.5;
		text-align: center;
		color: black;
		background-color: white;
	}

	.answer {
			font-family: arial;
		font-size: 20px;
		line-height: 1.5;
		text-align: left;
		color: black;
		background-color: white;
	}
	`
)

type AnkiPayload struct {
	Action  string         `json:"action"`
	Version int            `json:"version"`
	Params  map[string]any `json:"params"`
}

type AnkiResponse[T any] struct {
	Result T      `json:"result"`
	Error  string `json:"error"`
}

type MP4ToWebmConversionJob struct {
	InputPath  string
	OutputPath string
}

type MoveDataCard struct {
	CharacterName string
	CommandName   string
	VideoPath     string
	FramesOnBlock string
	FramesOnHit   string
	HitLevel      string
	Notes         string
}

func ConvertMp4ToWebm(inputPath string, outputPath string) error {
	cmd := exec.Command(
		"ffmpeg",
		"-i", inputPath,
		outputPath,
	)

	return cmd.Run()
}

func CreateCard(deckName string, data *MoveDataCard) error {
	request := AnkiPayload{
		Action:  "addNote",
		Version: 6,
		Params: map[string]any{
			"note": map[string]any{
				"deckName":  deckName,
				"modelName": OKI_SCRAPER_NOTE_TYPE_NAME,

				"fields": map[string]string{
					"Move Command":    data.CommandName,
					"Frames On Block": data.FramesOnBlock,
					"Frames On Hit":   data.FramesOnHit,
					"Hit Level":       data.HitLevel,
					"Notes":           data.Notes,
				},

				"video": []map[string]any{
					{
						"filename": filepath.Base(data.VideoPath),
						"path":     data.VideoPath,
						"fields": []string{
							"Video",
						},
					},
				},

				"tags": []string{
					"tekken8",
					data.CharacterName,
					"okiscraper",
				},
			},
		},
	}

	response, err := executeAnkiRequest[int64](request)
	if err != nil {
		return err
	}

	fmt.Println("Created note:", response.Result)

	return nil
}

func CreateDeck(deckName string) (int, error) {
	request := AnkiPayload{
		Action:  "createDeck",
		Version: 6,
		Params: map[string]any{
			"deck": deckName,
		},
	}

	response, err := executeAnkiRequest[int](request)
	if err != nil {
		return -1, err
	}

	return response.Result, nil
}

func GetNoteTypes() ([]string, error) {
	request := AnkiPayload{
		Action:  "modelNames",
		Version: 6,
		Params:  map[string]any{},
	}

	response, err := executeAnkiRequest[[]string](request)
	if err != nil {
		return nil, err
	}

	noteTypes := response.Result

	return noteTypes, nil
}

func CreateNoteType(noteTypeName string) error {
	request := AnkiPayload{
		Action:  "createModel",
		Version: 6,
		Params: map[string]any{
			"modelName": noteTypeName,
			"inOrderFields": []string{
				"Move Command",
				"Video",
				"Frames On Block",
				"Frames On Hit",
				"Hit Level",
				"Notes",
			},
			"css": okiScraperCardStylingTemplate,
			"cardTemplates": []map[string]string{
				{
					"Name":  "Card 1",
					"Front": "{{Video}}<br>{{Move Command}}",
					"Back":  okiScraperBackCardTemplate,
				},
			},
		},
	}

	_, err := executeAnkiRequest[any](request)
	if err != nil {
		return err
	}

	fmt.Printf("Anki Note Type [%s] created successfully\n\n", noteTypeName)

	return nil
}

func executeAnkiRequest[T any](request AnkiPayload) (*AnkiResponse[T], error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	response, err := http.Post(
		"http://localhost:8765",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// Parse response
	var result AnkiResponse[T]

	err = json.NewDecoder(response.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	// Check AnkiConnect error
	if result.Error != "" {
		return nil, fmt.Errorf("AnkiConnect error: %s", result.Error)
	}

	return &result, nil
}
