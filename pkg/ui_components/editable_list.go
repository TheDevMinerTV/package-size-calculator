package ui_components

import (
	"errors"
	"fmt"

	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
)

var (
	ErrRetry = errors.New("next")
)

type EditableList[T fmt.Stringer] struct {
	items     []*itemContainer[T]
	title     string
	convertFn StringToItemConvertFunc[T]
}

func NewEditableList[T Item](title string, convertFn StringToItemConvertFunc[T]) *EditableList[T] {
	return &EditableList[T]{
		title:     fmt.Sprintf("%s (press Enter to continue)", title),
		items:     []*itemContainer[T]{},
		convertFn: convertFn,
	}
}

func (s EditableList[T]) Run() ([]T, error) {
	selected, err := s.selectItems()
	if err != nil {
		return nil, err
	}

	mappedSelected := make([]T, len(selected))
	for i, item := range selected {
		mappedSelected[i] = item.Data
	}

	fmt.Printf("%s %s\n", color.GreenString("✔"), color.HiBlackString("%s: %s", s.title, joinItems(s.items)))

	return mappedSelected, nil
}

func (s *EditableList[T]) selectItems() ([]*itemContainer[T], error) {
	for _, item := range s.items {
		fmt.Printf("%s %s (%s)\n", color.GreenString("-"), color.HiWhiteString(item.Data.String()), color.HiBlackString(item.Label))
	}

	prompt := promptui.Prompt{Label: s.title, HideEntered: true}

	input, err := prompt.Run()
	if err != nil {
		return nil, err
	}

	if input == "" {
		return s.items, nil
	}

	item, err := s.convertFn(input)
	if err != nil && !errors.Is(err, ErrRetry) {
		return s.selectItems()
	} else if err == nil {
		s.items = append(s.items, &itemContainer[T]{Label: input, Data: item, Selected: false})
	}

	return s.selectItems()
}
