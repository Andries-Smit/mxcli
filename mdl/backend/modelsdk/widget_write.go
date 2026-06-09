// SPDX-License-Identifier: Apache-2.0

package modelsdkbackend

import (
	"fmt"

	"github.com/mendixlabs/mxcli/modelsdk/codec"
	"github.com/mendixlabs/mxcli/modelsdk/element"
	genPg "github.com/mendixlabs/mxcli/modelsdk/gen/pages"
	genTexts "github.com/mendixlabs/mxcli/modelsdk/gen/texts"
	"github.com/mendixlabs/mxcli/sdk/pages"
)

func init() {
	// Conditional-visibility / native-accessibility slots are null when unset, on
	// every widget that has them (verified against real page BSON).
	for _, t := range []string{"Forms$DivContainer", "Forms$DynamicText"} {
		codec.RegisterTypeDefaults(t, codec.TypeDefaults{
			NullFields: []string{"ConditionalVisibilitySettings", "NativeAccessibilitySettings"},
		})
		// Widgets nested in a Widgets list use the typed-array marker 2 when present.
		codec.RegisterListMarker(t, 2)
	}
	// A ClientTemplate's Parameters list is always emitted with marker 2, even empty
	// (unusual — most empty lists are marker 3).
	codec.RegisterTypeDefaults("Forms$ClientTemplate", codec.TypeDefaults{
		MandatoryListMarkers: map[string]int32{"Parameters": 2},
	})
}

// widgetToGen converts a model widget to its gen element, recursing into
// containers. Unsupported widget types are refused loudly (ADR-0005) so a page
// is never written with a silently-dropped widget.
func widgetToGen(w pages.Widget) (element.Element, error) {
	switch x := w.(type) {
	case *pages.Container:
		g := genPg.NewDivContainer()
		applyWidgetBase(g, &x.BaseWidget)
		g.SetRenderMode(orDefaultStr(string(x.RenderMode), "Div"))
		g.SetScreenReaderHidden(false)
		g.SetOnClickAction(noActionGen())
		for _, c := range x.Widgets {
			cg, err := widgetToGen(c)
			if err != nil {
				return nil, err
			}
			g.AddWidgets(cg)
		}
		return g, nil

	case *pages.DynamicText:
		g := genPg.NewDynamicText()
		applyWidgetBase(g, &x.BaseWidget)
		g.SetRenderMode(orDefaultStr(string(x.RenderMode), "Text"))
		g.SetNativeTextStyle("Text")
		g.SetContent(clientTemplateToGen(x.Content))
		return g, nil

	default:
		return nil, fmt.Errorf("CreatePage: widget %T not yet supported by the modelsdk engine — rerun with MXCLI_ENGINE=legacy", w)
	}
}

// widgetBaseGen is the shared setter surface of a gen widget element.
type widgetBaseGen interface {
	element.Element
	SetID(element.ID)
	SetName(string)
	SetAppearance(element.Element)
	SetTabIndex(int32)
}

// applyWidgetBase sets the fields every widget shares: identity, name, appearance
// (carrying class/style), and tab index. ConditionalVisibility/native
// accessibility are emitted null via the registered defaults.
func applyWidgetBase(g widgetBaseGen, b *pages.BaseWidget) {
	if b.ID != "" {
		g.SetID(element.ID(b.ID))
	}
	assignID(g)
	g.SetName(b.Name)
	g.SetAppearance(newAppearance(b.Class, b.Style))
	g.SetTabIndex(int32(b.TabIndex))
}

// newAppearance builds a Forms$Appearance with the given class/style (empty
// DesignProperties / dynamic classes).
func newAppearance(class, style string) *genPg.Appearance {
	a := genPg.NewAppearance()
	assignID(a)
	a.SetClass(class)
	a.SetStyle(style)
	a.SetDynamicClasses("")
	return a
}

// noActionGen builds the default Forms$NoAction (DisabledDuringExecution=true)
// used by widget OnClick slots that have no action.
func noActionGen() element.Element {
	a := genPg.NewNoClientAction() // emits $Type Forms$NoAction
	assignID(a)
	a.SetDisabledDuringExecution(true)
	return a
}

// clientTemplateToGen builds the Forms$ClientTemplate that backs a dynamic text
// (Template + Fallback are Texts$Text; Parameters stays empty for static text).
func clientTemplateToGen(ct *pages.ClientTemplate) element.Element {
	g := genPg.NewClientTemplate()
	assignID(g)
	if ct != nil {
		g.SetTemplate(captionToGen(ct.Template))
		g.SetFallback(captionToGen(ct.Fallback))
	} else {
		g.SetTemplate(genTexts.NewText())
		g.SetFallback(genTexts.NewText())
	}
	return g
}

// orDefaultStr returns s, or def when s is empty.
func orDefaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
