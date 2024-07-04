package ui_components

import (
	"github.com/manifoldco/promptui"
)

var yesNoOptions = []string{"Yes", "No"}

func YesNoPrompt(label string) (bool, error) {
	prompt := &promptui.Select{
		Label: label,
		Items: yesNoOptions,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return false, err
	}

	return idx == 0, nil
}
