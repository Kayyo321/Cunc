package main

import (
	"fmt"
	"os"

	"cunc/src/editor"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	p := tea.NewProgram(editor.InitialModel())
	var final_model tea.Model
	if model, err := p.Run(); err != nil {
		os.Exit(1)
	} else {
		final_model = model
	}

	editorModel := final_model.(editor.Model)
	to := editorModel.GetTo()
	subject := editorModel.GetSubject()
	body := editorModel.GetBody()

	fmt.Println("To:", to)
	fmt.Println("Subject:", subject)
	fmt.Println("Body:", body)
}
