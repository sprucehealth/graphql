package ast

// Description is a block description in markdown.
type Description struct {
	Loc  Location
	Text string
}

func (d *Description) GetLoc() Location {
	return d.Loc
}
