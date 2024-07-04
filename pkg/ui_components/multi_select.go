package ui_components

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
)

const (
	msItemSelectorIcon       = `{{if .Selected}}{{"●" | green}}{{else}}○{{end}}`
	msDoneSelectorIcon       = `{{if eq .Label "Done"}}✔{{else}} {{end}}`
	msDoneSelectorActiveIcon = `{{if eq .Label "Done"}}{{"✔" | green}}{{else}} {{end}}`
	msSelectorIcon           = `{{if eq .Label "Done"}}` + msDoneSelectorIcon + `{{else}}` + msItemSelectorIcon + `{{end}}`
	msSelectorActiveIcon     = `{{if eq .Label "Done"}}` + msDoneSelectorActiveIcon + `{{else}}` + msItemSelectorIcon + `{{end}}`
)

type MultiSelect[T fmt.Stringer] struct {
	items []*itemContainer[T]
	title string
}

func NewMultiSelect[T Item](title string, items []T) *MultiSelect[T] {
	mapped := make([]*itemContainer[T], len(items))
	for i, item := range items {
		mapped[i] = &itemContainer[T]{Label: item.String(), Data: item, Selected: false}
	}

	slices.SortStableFunc(mapped, func(a, b *itemContainer[T]) int {
		return cmp.Compare(a.Label, b.Label)
	})

	return &MultiSelect[T]{
		title: title,
		items: mapped,
	}
}

func (s MultiSelect[T]) Run() ([]T, error) {
	selected, err := s.selectItems(0, s.items)
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

func (s *MultiSelect[T]) selectItems(selectedPos int, allItems []*itemContainer[T]) ([]*itemContainer[T], error) {
	const doneID = "Done"
	if len(allItems) > 0 && allItems[0].Label != doneID {
		var items = []*itemContainer[T]{{Label: doneID}}
		allItems = append(items, allItems...)
	}

	templates := &promptui.SelectTemplates{
		Active:   "→ " + msSelectorActiveIcon + " {{ .Label }}",
		Inactive: "  " + msSelectorIcon + " {{ .Label | cyan }}",
	}

	prompt := promptui.Select{
		Label:     s.title,
		Items:     allItems,
		Templates: templates,
		Size:      int(math.Min(float64(len(allItems)), 16)),
		CursorPos: selectedPos,
		Searcher: func(input string, index int) bool {
			item := allItems[index]
			name := item.Label
			return strings.Contains(strings.ToLower(name), strings.ToLower(input))
		},
		HideSelected: true,
	}

	selectionIdx, _, err := prompt.Run()
	if err != nil {
		return nil, err
	}

	chosenItem := allItems[selectionIdx]

	if chosenItem.Label != doneID {
		chosenItem.Selected = !chosenItem.Selected
		return s.selectItems(selectionIdx, allItems)
	}

	var selectedItems []*itemContainer[T]
	for _, i := range allItems {
		if i.Selected {
			selectedItems = append(selectedItems, i)
		}
	}

	return selectedItems, nil
}
