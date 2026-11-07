package anki

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"okiscraper/internal/models"
)

const (
	OKI_SCRAPER_NOTE_TYPE_NAME = "okiscraper-tekken8"
	ankiConnectURL             = "http://localhost:8765"

	okiScraperFrontCardTemplate = "{{Video}}"
	okiScraperBackCardTemplate  = `
	{{FrontSide}}

	<hr id=answer>

	<div class="answer">
		<b>{{Move Command}}</b> <br><br>
		<b>Startup:</b> {{Startup}} <br><br>
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

	video {
		max-width: 100%;
		height: auto;
		display: block;
		margin: 0 auto;
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

func CreateCard(deckName string, characterName string, mv models.Move, videoPath string) error {
	filename, err := storeMediaFile(filepath.Base(videoPath), videoPath)
	if err != nil {
		return err
	}

	// Embed the video inline in the field so it autoplays/loops directly in the
	// card (rather than a [sound:...] popup). The <source src="..."> tag is
	// referenced from the field on purpose: Anki's media checker only scans note
	// fields for media references, so this keeps the .webm from being reported
	// as unused and deleted.
	videoHTML := fmt.Sprintf(
		`<video autoplay muted loop playsinline><source src="%s" type="video/webm"></video>`,
		filename,
	)

	request := AnkiPayload{
		Action:  "addNote",
		Version: 6,
		Params: map[string]any{
			"note": map[string]any{
				"deckName":  deckName,
				"modelName": OKI_SCRAPER_NOTE_TYPE_NAME,

				// Scope duplicate detection to this character's deck. The note
				// type is shared across all characters and its first field
				// ("Move Command") is generic notation (e.g. "df+2") that repeats
				// across characters. Without this, AnkiConnect's default global
				// check rejects a move if any other character already has the same
				// notation.
				"options": map[string]any{
					"duplicateScope": "deck",
				},

				"fields": map[string]string{
					"Move Command":    mv.Command,
					"Video":           videoHTML,
					"Startup":         mv.Startup,
					"Frames On Block": mv.FramesOnBlock,
					"Frames On Hit":   mv.FramesOnHit,
					"Hit Level":       mv.HitLevel,
					"Notes":           mv.Notes,
				},

				"tags": []string{
					"tekken8",
					characterName,
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

// storeMediaFile copies a file into Anki's media collection via the AnkiConnect
// storeMediaFile action and returns the filename as stored (Anki may rename it
// to avoid collisions). Unlike the `video` field attachment, this does not wrap
// the value in [sound:...].
func storeMediaFile(filename, path string) (string, error) {
	request := AnkiPayload{
		Action:  "storeMediaFile",
		Version: 6,
		Params: map[string]any{
			"filename": filename,
			"path":     path,
		},
	}

	response, err := executeAnkiRequest[string](request)
	if err != nil {
		return "", err
	}

	return response.Result, nil
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
				"Startup",
				"Frames On Block",
				"Frames On Hit",
				"Hit Level",
				"Notes",
			},
			"css": okiScraperCardStylingTemplate,
			"cardTemplates": []map[string]string{
				{
					"Name":  "Card 1",
					"Front": okiScraperFrontCardTemplate,
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
		ankiConnectURL,
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("could not reach AnkiConnect at %s - is Anki open with the AnkiConnect add-on installed? (%w)", ankiConnectURL, err)
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
