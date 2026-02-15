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

func view_drafts() {
	p := tea.NewProgram(drafts.InitialModel())
	if model_result, err := p.Run(); err != nil {
		os.Exit(1)
	} else {
		drafts_model := model_result.(drafts.Model)
		if draft := drafts_model.GetSelectedDraft(); draft != nil {
			// Load the selected draft in the editor
			editor_model := editor.LoadDraft(draft.ID, draft.To, draft.Subject, draft.Body)
			editor_program := tea.NewProgram(editor_model)
			if _, err := editor_program.Run(); err != nil {
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

		"-drafts": view_drafts,
		"-d":      view_drafts,
	}

	if len(os.Args) != 2 {
		usage()
		os.Exit(1)
	}

	mode := os.Args[1]
	if handler, ok := modes[mode]; ok {
		handler()
	} else {
		fmt.Fprintf(os.Stderr, "Unknown mode: '%s'\n", mode)

		usage()
	}
}
