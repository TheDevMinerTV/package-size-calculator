package ui_components

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
)

type EditableList[T fmt.Stringer] struct {
	items     []*itemContainer[T]
	title     string
	convertFn StringToItemConvertFunc[T]
}

func NewEditableList[T Item](title string, convertFn StringToItemConvertFunc[T]) *EditableList[T] {
	return &EditableList[T]{
		title:     title,
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
	prompt := promptui.Prompt{Label: s.title}
	packageName, err := prompt.Run()
	if err != nil {
		return nil, err
	}

	if packageName != "" {
		item, err := s.convertFn(packageName)
		if err != nil {
			return nil, err
		}

		s.items = append(s.items, &itemContainer[T]{Label: item.String(), Data: item, Selected: false})

		return s.selectItems()
	}

	return s.items, nil
}
