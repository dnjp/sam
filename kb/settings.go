package kb

import "strings"

type Filetype struct {
	Name         string
	Extensions   []string
	Comment      string
	Tabwidth     int
	Tabexpand    bool
	commentparts []string
}

var Filetypes = func() map[string]Filetype {
	fm := make(map[string]Filetype)
	for _, ft := range filetypes {
		for _, ext := range ft.Extensions {
			fm[ext] = ft
		}
	}
	return fm
}()

var DefaultFiletype = Filetype{
	Name:       "unknown",
	Extensions: []string{},
	Comment:    "# ",
	Tabwidth:   8,
	Tabexpand:  false,
}

func FindFiletype(filename string) (Filetype, bool) {
	ext := extension(filename)
	ft, ok := Filetypes[ext]
	if !ok {
		return DefaultFiletype, false
	}
	return ft, true
}

func (ft Filetype) HasComment(line string) bool {
	multipart := len(ft.commentParts()) > 1
	startcom := ft.commentStart()
	endcom := ft.commentEnd()
	comment := ft.Comment
	if multipart {
		comment = startcom
	}
	if strings.Contains(line, comment) {
		if multipart && !strings.Contains(line, endcom) {
			return false
		}
		return true
	}
	return false
}

func (ft Filetype) commentParts() []string {
	if len(ft.commentparts) == 0 {
		ft.commentparts = strings.Split(strings.TrimSuffix(ft.Comment, " "), " ")
	}
	return ft.commentparts
}

func (ft Filetype) commentStart() string {
	parts := ft.commentParts()
	start := ""
	if len(parts) >= 1 {
		start = parts[0]
	}
	if len(start) > 1 && start[len(start)-1] != ' ' {
		start += " "
	}
	return start
}

func (ft Filetype) commentEnd() string {
	parts := ft.commentParts()
	end := ""
	if len(parts) >= 2 {
		end = parts[1]
	}
	if len(end) > 1 && end[0] != ' ' {
		end = " " + end
	}
	return end
}

func extension(filename string) string {
	fn := filename
	path := strings.Split(filename, "/")
	fn = path[len(path)-1]
	pts := strings.Split(fn, ".")
	if len(pts) == 1 {
		return fn
	}
	return "." + pts[len(pts)-1]
}

var filetypes = []Filetype{
	{
		Name:       "c",
		Extensions: []string{".c", ".h"},
		Comment:    "/* */",
		Tabwidth:   8,
		Tabexpand:  false,
	},
	{
		Name:       "c++",
		Extensions: []string{".cc", ".cpp", ".hpp", ".cxx", ".hxx"},
		Comment:    "// ",
		Tabwidth:   2,
		Tabexpand:  true,
	},
	{
		Name:       "html",
		Extensions: []string{".html"},
		Comment:    "<!-- -->",
		Tabwidth:   2,
		Tabexpand:  true,
	},
	{
		Name:       "java",
		Extensions: []string{".java"},
		Comment:    "// ",
		Tabwidth:   2,
		Tabexpand:  true,
	},
	{
		Name:       "javascript",
		Extensions: []string{".js", ".ts"},
		Comment:    "// ",
		Tabwidth:   2,
		Tabexpand:  true,
	},
	{
		Name:       "json",
		Extensions: []string{".json"},
		Comment:    "",
		Tabwidth:   2,
		Tabexpand:  true,
	},
	{
		Name:       "go",
		Extensions: []string{".go"},
		Comment:    "// ",
		Tabwidth:   8,
		Tabexpand:  false,
	},
	{
		Name:       "shell",
		Extensions: []string{".sh", ".rc", ".bash", ".zsh"},
		Comment:    "# ",
		Tabwidth:   8,
		Tabexpand:  false,
	},
	{
		Name:       "terraform",
		Extensions: []string{".tf"},
		Comment:    "# ",
		Tabwidth:   2,
		Tabexpand:  true,
	},
	{
		Name:       "yaml",
		Extensions: []string{".yaml"},
		Comment:    "# ",
		Tabwidth:   2,
		Tabexpand:  true,
	},
}
