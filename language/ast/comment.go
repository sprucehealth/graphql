package ast

// Comment is a line or block comment
type Comment struct {
	Loc  Location
	Text string
}

func (c *Comment) GetLoc() Location {
	return c.Loc
}

// CommentGroup represents a sequence of comments with no other tokens and no empty lines between.
type CommentGroup struct {
	Loc  Location
	List []*Comment
}

func (c *CommentGroup) GetLoc() Location {
	return c.Loc
}
