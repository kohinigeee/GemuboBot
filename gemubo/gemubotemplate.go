package gemubo

type Template struct {
	Name    string
	Content string
}

func NewTemplate(name string, content string) *Template {
	return &Template{
		Name:    name,
		Content: content,
	}
}
