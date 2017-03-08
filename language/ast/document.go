package ast

// Document implements Node
type Document struct {
	Loc         Location
	Definitions []Node
	Comments    []*CommentGroup
}

func (node *Document) GetLoc() Location {
	return node.Loc
}
