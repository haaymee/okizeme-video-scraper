package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
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

	return tekkenCharacterName, selectedOptions, nil
}
