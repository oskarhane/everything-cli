package label

import (
	"fmt"

	"github.com/spf13/pflag"

	gmail "google.golang.org/api/gmail/v1"
)

// addLabelFlags registers the optional write flags shared by create and
// update. create additionally takes the name positionally; update adds
// --name, so applyLabelFlags only reads "name" when the flag exists.
func addLabelFlags(f *pflag.FlagSet) {
	f.String("color-text", "", "Label text color as #rrggbb")
	f.String("color-bg", "", "Label background color as #rrggbb")
	f.String("label-list-visibility", "", "Show the label in the label list: labelShow or labelHide")
	f.String("message-list-visibility", "", "Show messages with the label in the message list: show or hide")
}

// applyLabelFlags writes the set flags on f into label, leaving unset fields
// alone so update sends a partial body. name, when non-empty, seeds
// label.Name (create's positional argument).
func applyLabelFlags(label *gmail.Label, f *pflag.FlagSet, name string) error {
	if name != "" {
		label.Name = name
	}
	if f.Changed("name") {
		v, _ := f.GetString("name")
		label.Name = v
	}
	colorText, _ := f.GetString("color-text")
	colorBg, _ := f.GetString("color-bg")
	if colorText != "" || colorBg != "" {
		label.Color = &gmail.LabelColor{TextColor: colorText, BackgroundColor: colorBg}
	}
	if v, _ := f.GetString("label-list-visibility"); v != "" {
		if v != "labelShow" && v != "labelHide" {
			return fmt.Errorf("invalid --label-list-visibility %q: expected labelShow or labelHide", v)
		}
		label.LabelListVisibility = v
	}
	if v, _ := f.GetString("message-list-visibility"); v != "" {
		if v != "show" && v != "hide" {
			return fmt.Errorf("invalid --message-list-visibility %q: expected show or hide", v)
		}
		label.MessageListVisibility = v
	}
	return nil
}

// anyLabelFlagChanged reports whether any of the optional write flags was set
// on f, so update can refuse an empty modification.
func anyLabelFlagChanged(f *pflag.FlagSet) bool {
	for _, name := range []string{"name", "color-text", "color-bg", "label-list-visibility", "message-list-visibility"} {
		if f.Changed(name) {
			return true
		}
	}
	return false
}
