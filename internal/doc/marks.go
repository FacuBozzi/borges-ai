package doc

// Mark is a bitmask of inline text styles. Multiple marks can apply to the
// same run (e.g. bold + italic).
type Mark uint8

const (
	MarkBold Mark = 1 << iota
	MarkItalic
	MarkUnderline
	MarkCode
	MarkStrike
)

func (m Mark) Has(o Mark) bool { return m&o != 0 }
func (m Mark) With(o Mark) Mark { return m | o }
func (m Mark) Without(o Mark) Mark { return m &^ o }
func (m Mark) Toggle(o Mark) Mark { return m ^ o }
