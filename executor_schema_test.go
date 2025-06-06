package graphql_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/sprucehealth/graphql"
	"github.com/sprucehealth/graphql/testutil"
)

// TODO: have a separate package for other tests for eg `parser`
// maybe for:
// - tests that supposed to be black-boxed (no reason to access private identifiers)
// - tests that create internal tests structs, we might not want to pollute the package with too many test structs

type testPic struct {
	URL    string `json:"url"`
	Width  string `json:"width"`
	Height string `json:"height"`
}

type testPicFn func(width, height string) *testPic

type testAuthor struct {
	ID            int          `json:"id"`
	Name          string       `json:"name"`
	Pic           testPicFn    `json:"pic"`
	RecentArticle *testArticle `json:"recentArticle"`
}
type testArticle struct {
	ID          string      `json:"id"`
	IsPublished string      `json:"isPublished"`
	Author      *testAuthor `json:"author"`
	Title       string      `json:"title"`
	Body        string      `json:"body"`
	Hidden      string      `json:"hidden"`
	Keywords    []any       `json:"keywords"`
}

func getPic(id int, width, height string) *testPic {
	return &testPic{
		URL:    fmt.Sprintf("cdn://%v", id),
		Width:  width,
		Height: height,
	}
}

var johnSmith *testAuthor

func article(id any) *testArticle {
	return &testArticle{
		ID:          fmt.Sprintf("%v", id),
		IsPublished: "true",
		Author:      johnSmith,
		Title:       fmt.Sprintf("My Article %v", id),
		Body:        "This is a post",
		Hidden:      "This data is not exposed in the schema",
		Keywords: []any{
			"foo", "bar", 1, true, nil,
		},
	}
}

func TestExecutesUsingAComplexSchema(t *testing.T) {
	johnSmith = &testAuthor{
		ID:   123,
		Name: "John Smith",
		Pic: func(width string, height string) *testPic {
			return getPic(123, width, height)
		},
		RecentArticle: article("1"),
	}

	blogImage := graphql.NewObject(graphql.ObjectConfig{
		Name: "Image",
		Fields: graphql.Fields{
			"url": &graphql.Field{
				Type: graphql.String,
			},
			"width": &graphql.Field{
				Type: graphql.Int,
			},
			"height": &graphql.Field{
				Type: graphql.Int,
			},
		},
	})
	blogAuthor := graphql.NewObject(graphql.ObjectConfig{
		Name: "Author",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.String,
			},
			"name": &graphql.Field{
				Type: graphql.String,
			},
			"pic": &graphql.Field{
				Type: blogImage,
				Args: graphql.FieldConfigArgument{
					"width": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"height": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
				},
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					if author, ok := p.Source.(*testAuthor); ok {
						width := fmt.Sprintf("%v", p.Args["width"])
						height := fmt.Sprintf("%v", p.Args["height"])
						return author.Pic(width, height), nil
					}
					return nil, nil
				},
			},
			"recentArticle": &graphql.Field{},
		},
	})
	blogArticle := graphql.NewObject(graphql.ObjectConfig{
		Name: "Article",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.NewNonNull(graphql.String),
			},
			"isPublished": &graphql.Field{
				Type: graphql.Boolean,
			},
			"author": &graphql.Field{
				Type: blogAuthor,
			},
			"title": &graphql.Field{
				Type: graphql.String,
			},
			"body": &graphql.Field{
				Type: graphql.String,
			},
			"keywords": &graphql.Field{
				Type: graphql.NewList(graphql.String),
			},
		},
	})

	blogAuthor.AddFieldConfig("recentArticle", &graphql.Field{
		Type: blogArticle,
	})

	blogQuery := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"article": &graphql.Field{
				Type: blogArticle,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.ID,
					},
				},
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					id := p.Args["id"]
					return article(id), nil
				},
			},
			"feed": &graphql.Field{
				Type: graphql.NewList(blogArticle),
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					return []*testArticle{
						article(1),
						article(2),
						article(3),
						article(4),
						article(5),
						article(6),
						article(7),
						article(8),
						article(9),
						article(10),
					}, nil
				},
			},
		},
	})

	blogSchema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: blogQuery,
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	request := `
      {
        feed {
          id,
          title
        },
        article(id: "1") {
          ...articleFields,
          author {
            id,
            name,
            pic(width: 640, height: 480) {
              url,
              width,
              height
            },
            recentArticle {
              ...articleFields,
              keywords
            }
          }
        }
      }

      fragment articleFields on Article {
        id,
        isPublished,
        title,
        body,
        hidden,
        notdefined
      }
	`

	expected := &graphql.Result{
		Data: map[string]any{
			"article": map[string]any{
				"title": "My Article 1",
				"body":  "This is a post",
				"author": map[string]any{
					"id":   "123",
					"name": "John Smith",
					"pic": map[string]any{
						"url":    "cdn://123",
						"width":  640,
						"height": 480,
					},
					"recentArticle": map[string]any{
						"id":          "1",
						"isPublished": bool(true),
						"title":       "My Article 1",
						"body":        "This is a post",
						"keywords": []any{
							"foo",
							"bar",
							"1",
							"true",
							nil,
						},
					},
				},
				"id":          "1",
				"isPublished": bool(true),
			},
			"feed": []any{
				map[string]any{
					"id":    "1",
					"title": "My Article 1",
				},
				map[string]any{
					"id":    "2",
					"title": "My Article 2",
				},
				map[string]any{
					"id":    "3",
					"title": "My Article 3",
				},
				map[string]any{
					"id":    "4",
					"title": "My Article 4",
				},
				map[string]any{
					"id":    "5",
					"title": "My Article 5",
				},
				map[string]any{
					"id":    "6",
					"title": "My Article 6",
				},
				map[string]any{
					"id":    "7",
					"title": "My Article 7",
				},
				map[string]any{
					"id":    "8",
					"title": "My Article 8",
				},
				map[string]any{
					"id":    "9",
					"title": "My Article 9",
				},
				map[string]any{
					"id":    "10",
					"title": "My Article 10",
				},
			},
		},
	}

	// parse query
	ast := testutil.TestParse(t, request)

	// execute
	ep := graphql.ExecuteParams{
		Schema: blogSchema,
		AST:    ast,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}
