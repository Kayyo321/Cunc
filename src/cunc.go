package main

import (
	"cunc/src/drafts"
	"cunc/src/editor"
	"cunc/src/inbox"
	"cunc/src/settings"
	"fmt"
	"os"
	"strconv"

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
	fmt.Println("     -settings : view and edit your settings")
	fmt.Println("     -s          including your email/password for logging in!")
	fmt.Println()
	fmt.Println("     -inbox : view your current inbox")
	fmt.Println("     -i")
	fmt.Println()
}

func view_drafts() {
	p := tea.NewProgram(drafts.InitialModel())
	if model_result, err := p.Run(); err != nil {
		os.Exit(1)
	} else {
		drafts_model := model_result.(drafts.Model)
		if drafts_model.Action == "select" && drafts_model.GetSelectedDraft() != nil {
			draft := drafts_model.GetSelectedDraft()
			// Load the selected draft in the editor
			editor_model := editor.LoadDraft(draft.ID, draft.To, draft.Subject, draft.Body)
			editor_program := tea.NewProgram(editor_model)
			if _, err := editor_program.Run(); err != nil {
				os.Exit(1)
			}
		}
	}
}

func view_settings() {
	p := tea.NewProgram(settings.InitialModel())
	if _, err := p.Run(); err != nil {
		os.Exit(1)
	}
}

func view_inbox() {
	// Try to read credentials from settings and fetch real emails via IMAP
	sm := settings.InitialModel()
	user := sm.GetSetting("email")
	pass := sm.GetSetting("2fa app-password")
	if pass == "" {
		pass = sm.GetSetting("password")
	}

	// Determine per-page from settings
	perPage := 10
	if s := sm.GetSetting("emails per page"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			perPage = v
		}
	}

	var inbox_model inbox.Model
	// If credentials present, start with loading=true and let the inbox model fetch
	if user != "" && pass != "" {
		inbox_model = inbox.InitialModel([]inbox.Email{}, perPage, true, user, pass, 50)
	} else {
		// No credentials: provide sample emails and no loading
		emails := []inbox.Email{
			{ID: "1", From: "alice@example.com", Subject: "Hello", Body: "Hi there! How are you?"},
			{ID: "2", From: "bob@example.com", Subject: "Meeting", Body: "Don't forget our meeting tomorrow at 10am."},
			{ID: "3", From: "carol@example.com", Subject: "Greetings", Body: "Just wanted to say hi."},
		}
		inbox_model = inbox.InitialModel(emails, perPage, false, "", "", 0)
	}

	// Run Bubble Tea program
	p := tea.NewProgram(inbox_model)
	if _, err := p.Run(); err != nil {
		os.Exit(1)
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

		"-settings": view_settings,
		"-s":        view_settings,

		"-inbox": view_inbox,
		"-i":     view_inbox,
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
