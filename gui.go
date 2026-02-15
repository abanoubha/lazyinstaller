package main

import (
	"fmt"
	"image/color"
	"log"
	"os"
	"strings"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type UI struct {
	Theme  *material.Theme
	Window *app.Window

	// State
	DetectedPMs  []packageManager
	Packages     []Package
	FilteredPkgs []Package

	// Widgets
	PMList  layout.List
	PkgList layout.List

	PMSelectBtns []*widget.Clickable
	SelectedPM   string // "All" or PM name

	FilterDropdown *widget.Bool // Placeholder, maybe use Enum or just buttons
	FilterOption   string       // "Installed", "Update Available", "All"

	FilterButtons []struct {
		Label  string
		Value  string
		Widget *widget.Clickable
	}

	SearchInput widget.Editor

	// Cached operations
	Ops op.Ops
}

func runGUI(pms []packageManager, initialPkgs []Package) {
	go func() {
		w := new(app.Window)
		w.Option(app.Title("LazyInstaller"), app.Size(unit.Dp(1000), unit.Dp(700)))
		if err := loop(w, pms, initialPkgs); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func loop(w *app.Window, pms []packageManager, pkgs []Package) error {
	th := material.NewTheme()

	// Prepare PM Buttons
	pmBtns := make([]*widget.Clickable, len(pms)+1) // +1 for "All"
	for i := range pmBtns {
		pmBtns[i] = new(widget.Clickable)
	}

	// Prepare Filter Buttons
	filterBtns := []struct {
		Label  string
		Value  string
		Widget *widget.Clickable
	}{
		{"Installed", "Installed", new(widget.Clickable)},
		{"Update Available", "Update Available", new(widget.Clickable)},
		{"All", "All", new(widget.Clickable)},
	}

	ui := &UI{
		Theme:         th,
		Window:        w,
		DetectedPMs:   pms,
		Packages:      pkgs,
		FilteredPkgs:  pkgs,
		PMList:        layout.List{Axis: layout.Vertical},
		PkgList:       layout.List{Axis: layout.Vertical},
		PMSelectBtns:  pmBtns,
		SelectedPM:    "All",
		FilterOption:  "Installed",
		FilterButtons: filterBtns,
	}
	ui.SearchInput.SingleLine = true
	ui.SearchInput.Submit = true

	// Initial Filter
	ui.applyFilters()

	for {
		e := w.Event()
		switch e := e.(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ui.Ops, e)
			ui.Layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (ui *UI) applyFilters() {
	var res []Package
	query := strings.ToLower(normalizeText(ui.SearchInput.Text()))

	for _, p := range ui.Packages {
		// 1. PM Filter
		if ui.SelectedPM != "All" && p.Manager != ui.SelectedPM {
			// Some managers have variants like "apt/dpkg" -> "apt" logic handled in commands,
			// but here we just match string.
			// detectPM() returns 'wrapperName' which might be "apt".
			// pkg.Manager coming from main.go might be "apt/dpkg" or similar.
			// We need a normalized check.
			if !strings.Contains(p.Manager, ui.SelectedPM) {
				continue
			}
		}

		// 2. Status Filter
		if ui.FilterOption == "Installed" && !p.IsInstalled {
			continue
		}
		if ui.FilterOption == "Update Available" && !p.IsUpgradable {
			continue
		}

		// 3. Search
		if query != "" {
			if !strings.Contains(strings.ToLower(p.Name), query) {
				continue
			}
		}

		res = append(res, p)
	}
	ui.FilteredPkgs = res
}

func (ui *UI) Layout(gtx layout.Context) layout.Dimensions {
	// Process inputs
	if ui.PMSelectBtns[0].Clicked(gtx) {
		ui.SelectedPM = "All"
		ui.applyFilters()
	}
	for i, pm := range ui.DetectedPMs {
		if ui.PMSelectBtns[i+1].Clicked(gtx) {
			ui.SelectedPM = pm.Name
			ui.applyFilters()
		}
	}

	for _, buf := range ui.FilterButtons {
		if buf.Widget.Clicked(gtx) {
			ui.FilterOption = buf.Value
			ui.applyFilters()
		}
	}

	// for _, e := range ui.SearchInput.Events() {
	// 	if _, ok := e.(widget.SubmitEvent); ok {
	// 		ui.applyFilters()
	// 	}
	// }
	// Also update on text change? Gio Editor doesn't have explicit "OnChange" event in standard queue,
	// but we can check text diff or just re-apply every frame if efficient (not efficient for cached lists).
	// Simplest: re-apply if text changes.
	// For now, let's rely on Submit or check changes.

	// Layout: Vertical (Top Bar, Content)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.layoutTopBar(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			// Content: Horizontal (Sidebar, Pkg List)
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.layoutSidebar(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return ui.layoutPkgList(gtx)
				}),
			)
		}),
	)
}

func (ui *UI) layoutTopBar(gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				editor := material.Editor(ui.Theme, &ui.SearchInput, "Search packages...")
				editor.Editor.SingleLine = true
				border := widget.Border{Color: color.NRGBA{A: 0xff, R: 0xcc, G: 0xcc, B: 0xcc}, Width: unit.Dp(1), CornerRadius: unit.Dp(4)}
				return border.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(8)).Layout(gtx, editor.Layout)
				})
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				// Filter Buttons
				var children []layout.FlexChild
				for _, fb := range ui.FilterButtons {
					f := fb // capture loop var
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(ui.Theme, f.Widget, f.Label)
						if ui.FilterOption == f.Value {
							btn.Background = color.NRGBA{R: 0x88, G: 0x88, B: 0xff, A: 0xff}
						} else {
							btn.Background = color.NRGBA{R: 0xdd, G: 0xdd, B: 0xdd, A: 0xff}
							btn.Color = color.NRGBA{A: 0xff}
						}
						return layout.Inset{Right: unit.Dp(4)}.Layout(gtx, btn.Layout)
					}))
				}
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
			}),
		)
	})
}

func (ui *UI) layoutSidebar(gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			// Sidebar Background
			paint.FillShape(gtx.Ops, color.NRGBA{R: 0xf0, G: 0xf0, B: 0xf0, A: 0xff}, clip.Rect{Max: gtx.Constraints.Min}.Op())
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return ui.PMList.Layout(gtx, len(ui.DetectedPMs)+1, func(gtx layout.Context, index int) layout.Dimensions {
				var label string
				var btn *widget.Clickable
				if index == 0 {
					label = "All"
					btn = ui.PMSelectBtns[0]
				} else {
					label = ui.DetectedPMs[index-1].Name
					btn = ui.PMSelectBtns[index]
				}

				return material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body1(ui.Theme, label)
						if ui.SelectedPM == label {
							lbl.Font.Weight = font.Bold
						}
						return lbl.Layout(gtx)
					})
				})
			})
		}),
	)
}

func (ui *UI) layoutPkgList(gtx layout.Context) layout.Dimensions {
	return ui.PkgList.Layout(gtx, len(ui.FilteredPkgs), func(gtx layout.Context, index int) layout.Dimensions {
		pkg := ui.FilteredPkgs[index]
		return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return material.Body1(ui.Theme, pkg.Name).Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return material.Caption(ui.Theme, fmt.Sprintf("%s • %s", pkg.Manager, pkg.Version)).Layout(gtx)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					status := "Not Installed"
					if pkg.IsInstalled {
						status = "Installed"
					}
					return material.Caption(ui.Theme, status).Layout(gtx)
				}),
			)
		})
	})
}
