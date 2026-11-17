package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// The c9s theme uses the Severalnines product colours: deep navy surfaces
// with magenta/pink accents. Every text colour is chosen to stay readable on
// the navy background (WCAG contrast 4.5:1 or better).
//
//	role             hex      used for
//	background       #161a2f  whole screen
//	panel            #1d2344  column headers, input fields, buttons
//	selection        #3a47b8  selected table row (white text)
//	border           #4a54a8  box borders
//	magenta          #b80b7b  path bar, highlighted button (white text)
//	pink             #ff3fa5  keys, scopes, logo, headings, column titles
//	ink              #e8eaf6  normal text and table rows
//	label            #9ea6d8  header labels
//	dim              #7d84b2  completed / secondary text
//	ok               #3ddc97  success
//	warn             #ffb020  warning, busy, maintenance
//	bad              #ff5a5a  failed, offline, critical, errors
//
// Inline colour tags in strings ("[#ff3fa5]") use the same hex values.
var (
	colBg        = tcell.NewHexColor(0x161a2f)
	colPanel     = tcell.NewHexColor(0x1d2344)
	colSelection = tcell.NewHexColor(0x3a47b8)
	colBorder    = tcell.NewHexColor(0x4a54a8)
	colMagenta   = tcell.NewHexColor(0xb80b7b)
	colPink      = tcell.NewHexColor(0xff3fa5)
	colInk       = tcell.NewHexColor(0xe8eaf6)
	colLabel     = tcell.NewHexColor(0x9ea6d8)
	colDim       = tcell.NewHexColor(0x7d84b2)
	colOK        = tcell.NewHexColor(0x3ddc97)
	colWarn      = tcell.NewHexColor(0xffb020)
	colBad       = tcell.NewHexColor(0xff5a5a)
	colWhite     = tcell.NewHexColor(0xffffff)
)

// Colour names for tags built at run time ("[" + tagBad + "]").
const (
	tagInk  = "#e8eaf6"
	tagBad  = "#ff5a5a"
	tagWarn = "#ffb020"
)

// applyTheme sets the defaults every tview widget starts from.
func applyTheme() {
	tview.Styles = tview.Theme{
		PrimitiveBackgroundColor:    colBg,
		ContrastBackgroundColor:     colPanel,
		MoreContrastBackgroundColor: colSelection,
		BorderColor:                 colBorder,
		TitleColor:                  colInk,
		GraphicsColor:               colBorder,
		PrimaryTextColor:            colInk,
		SecondaryTextColor:          colPink,
		TertiaryTextColor:           colOK,
		InverseTextColor:            colWhite,
		ContrastSecondaryTextColor:  colPink,
	}
}

// styleForm gives dialogs the theme: labels in the label colour, fields and
// buttons on the panel colour, and the highlighted button in magenta.
func styleForm(form *tview.Form) {
	form.SetLabelColor(colLabel).
		SetFieldStyle(tcell.StyleDefault.Background(colPanel).Foreground(colInk)).
		SetButtonStyle(tcell.StyleDefault.Background(colPanel).Foreground(colInk)).
		SetButtonActivatedStyle(tcell.StyleDefault.Background(colMagenta).Foreground(colWhite).Bold(true))
}
