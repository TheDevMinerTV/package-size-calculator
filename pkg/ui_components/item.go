package ui_components

import (
	"slices"
	"strings"
)

type Item interface {
	String() string
}

type StringToItemConvertFunc[T Item] func(string) (T, error)

type itemContainer[T Item] struct {
	Label    string
	Data     T
	Selected bool
}

func joinItems[T Item](items []*itemContainer[T]) string {
	var selectedItems []string
	for _, i := range items {
		if i.Selected {
			selectedItems = append(selectedItems, i.Label)
		}
	}

	slices.Sort(selectedItems)

	return strings.Join(selectedItems, ", ")
}
