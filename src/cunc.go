package main

import (
	"cunc/src/drafts"
	"cunc/src/editor"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func usage() {
	fmt.Println("cunc <mode>")
	fmt.Println()
	fmt.Println("     -help : lets you read this page")
	fmt.Println("     -h")
	fmt.Println()
	fmt.Println("     -compose : begin writing a new email through this mode you can ")
	fmt.Println("     -c         save your current email as a draft")
	fmt.Println()
	fmt.Println("     -drafts : view and edit your saved drafts")
	fmt.Println("     -d")
	fmt.Println()
}

func viewDrafts() {
	p := tea.NewProgram(drafts.InitialModel())
	if model, err := p.Run(); err != nil {
		os.Exit(1)
	} else {
		draftsModel := model.(drafts.Model)
		if draft := draftsModel.GetSelectedDraft(); draft != nil {
			// Load the selected draft in the editor
			editorModel := editor.LoadDraft(draft.ID, draft.To, draft.Subject, draft.Body)
			editorProgram := tea.NewProgram(editorModel)
			if _, err := editorProgram.Run(); err != nil {
				os.Exit(1)
			}
		}
	}
}

func main() {
	modes := map[string]func(){
		"-help": usage,
		"-h":    usage,

		"-compose": editor.Compose,
		"-c":       editor.Compose,

		"-drafts": viewDrafts,
		"-d":      viewDrafts,
	}

	if len(os.Args) != 2 {
		usage()
		os.Exit(1)
	}

	mode := os.Args[1]
	if todo, ok := modes[mode]; ok {
		todo()
	} else {
		fmt.Fprintf(os.Stderr, "Unknown mode: '%s'\n", mode)

		usage()
	}
}
