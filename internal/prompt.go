package internal

import "github.com/manifoldco/promptui"

func RunSelect(s *promptui.Select) (int, string, error) {
	return s.Run()
}

func RunPrompt(p *promptui.Prompt) (string, error) {
	return p.Run()
}
