package ui_components

type String string

func (s String) String() string {
	return string(s)
}
