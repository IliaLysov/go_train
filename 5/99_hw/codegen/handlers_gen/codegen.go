// handlers_gen/codegen.go
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"unicode"
)

type APIValidator struct {
	Type      string
	Required  bool
	Paramname string
	Enum      []string
	Default   string
	Min       *int
	Max       *int
}

type APIField struct {
	Name      string
	Validator APIValidator
}

type APIMeta struct {
	URL    string `json:"url"`
	Auth   bool   `json:"auth"`
	Method string `json:"method"`
}

type APIHandler struct {
	APIMeta
	Name       string
	Receiver   string
	Fields     []APIField
	ParamsName string
}

type APIServeHTTP struct {
	Receiver string
	Handlers []APIHandler
}

func toLowerFirst(s string) string {
	if s == "" {
		return ""
	}
	rs := []rune(s)
	rs[0] = unicode.ToLower(rs[0])
	return string(rs)
}

func join(vals []string, sep string) string {
	return strings.Join(vals, sep)
}

var serveHTTPTpl = template.Must(template.New("serveHTTPTpl").Funcs(template.FuncMap{
	"lowerFirst": toLowerFirst,
	"join":       join,
}).Parse(`
func (api *{{.Receiver}}) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	{{- range .Handlers }}
	case "{{ .URL }}":
		api.handler{{ $.Receiver }}{{ .Name }}(w, r)
	{{- end }}
	default:
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "unknown method",
		})
		return
	}
}

{{- range .Handlers }}

func (api *{{.Receiver}}) handler{{.Receiver}}{{.Name}}(w http.ResponseWriter, r *http.Request) {
	{{- if .Method }}
	if r.Method != "{{ .Method }}" {
		w.WriteHeader(http.StatusNotAcceptable)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "bad method",
		})
		return
	}
	{{- end }}

	{{- if .Auth }}
	if r.Header.Get("X-Auth") != "100500" {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "unauthorized",
		})
		return
	}
	{{- end }}

	var params {{ .ParamsName }}

	{{- range .Fields }}
	//{{ .Name }}
	{
		paramName := "{{ if .Validator.Paramname }}{{ .Validator.Paramname }}{{ else }}{{ lowerFirst .Name }}{{ end }}"
		{{- if eq .Validator.Type "int" }}
		valStr := r.FormValue(paramName)
		{{- if .Validator.Default }}
		if valStr == "" {
			valStr = "{{ .Validator.Default }}"
		}
		{{- end }}
		{{- if .Validator.Required }}
		if valStr == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "{{ lowerFirst .Name }} must be not empty",
			})
			return
		}
		{{- end }}

		var valInt int
		if valStr == "" {
			valInt = 0
		} else {
			tmp, err := strconv.Atoi(valStr)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error": "{{ lowerFirst .Name }} must be int",
				})
				return
			}
			valInt = tmp
		}

		{{- if .Validator.Min }}
		if valInt < {{ .Validator.Min }} {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "{{ lowerFirst .Name }} must be >= {{ .Validator.Min }}",
			})
			return
		}
		{{- end }}
		{{- if .Validator.Max }}
		if valInt > {{ .Validator.Max }} {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "{{ lowerFirst .Name }} must be <= {{ .Validator.Max }}",
			})
			return
		}
		{{- end }}

		params.{{ .Name }} = valInt
		{{- else if eq .Validator.Type "string" }}
		val := r.FormValue(paramName)

		{{- if .Validator.Default }}
		if val == "" {
			val = "{{ .Validator.Default }}"
		}
		{{- end }}

		{{- if .Validator.Required }}
		if val == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "{{ lowerFirst .Name }} must be not empty",
			})
			return
		}
		{{- end }}

		{{- if .Validator.Min }}
		if len(val) < {{ .Validator.Min }} {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "{{ lowerFirst .Name }} len must be >= {{ .Validator.Min }}",
			})
			return
		}
		{{- end }}

		{{- if .Validator.Max }}
		if len(val) > {{ .Validator.Max }} {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "{{ lowerFirst .Name }} len must be <= {{ .Validator.Max }}",
			})
			return
		}
		{{- end }}

		{{- if .Validator.Enum }}
		allowed{{ .Name }} := map[string]bool{
			{{- range .Validator.Enum }}
			"{{ . }}": true,
			{{- end }}
		}
		if !allowed{{ .Name }}[val] {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "{{ lowerFirst .Name }} must be one of [{{ join .Validator.Enum ", " }}]",
			})
			return
		}
		{{- end }}

		params.{{ .Name }} = val
		{{- end }}
	}
	{{- end }}

	res, err := api.{{ .Name }}(r.Context(), params)
	if err != nil {
		switch e := err.(type) {
		case ApiError:
			w.WriteHeader(e.HTTPStatus)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": e.Err.Error(),
			})
			return
		case *ApiError:
			w.WriteHeader(e.HTTPStatus)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": e.Err.Error(),
			})
			return
		default:
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": err.Error(),
			})
			return
		}
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error":    "",
		"response": res,
	})
}
{{- end }}
`))

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("usage: %s <in.go> <out.go>", os.Args[0])
	}

	in := os.Args[1]
	outFile := os.Args[2]

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, in, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	out, err := os.Create(outFile)
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	fmt.Fprintf(out, "package %s\n\n", node.Name.Name)
	fmt.Fprintln(out, `import (`)
	fmt.Fprintln(out, `  "encoding/json"`)
	fmt.Fprintln(out, `  "net/http"`)
	fmt.Fprintln(out, `  "strconv"`)
	fmt.Fprintln(out, `)`)
	fmt.Fprintln(out)

	validators := make(map[string][]APIField)
	apis := make(map[string]*APIServeHTTP)

	for _, decl := range node.Decls {
		fun, ok := decl.(*ast.FuncDecl)
		if !ok || fun.Doc == nil {
			continue
		}

		for _, c := range fun.Doc.List {
			if !strings.HasPrefix(c.Text, "// apigen:api") {
				continue
			}

			if fun.Recv == nil || len(fun.Recv.List) != 1 {
				continue
			}

			star, ok := fun.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			receiverIdent, ok := star.X.(*ast.Ident)
			if !ok {
				continue
			}
			receiver := receiverIdent.Name

			var meta APIMeta
			jsonText := strings.TrimPrefix(c.Text, "// apigen:api ")
			if err := json.Unmarshal([]byte(jsonText), &meta); err != nil {
				log.Fatalf("cant parse apigen json for %s: %v", fun.Name.Name, err)
			}

			if fun.Type.Params == nil || len(fun.Type.Params.List) != 2 {
				continue
			}
			paramIdent, ok := fun.Type.Params.List[1].Type.(*ast.Ident)
			if !ok {
				continue
			}
			paramName := paramIdent.Name

			fields, ok := validators[paramName]
			if !ok {
				fields = findAndParseValidators(node, paramName)
				validators[paramName] = fields
			}

			h := APIHandler{
				APIMeta:    meta,
				Name:       fun.Name.Name,
				Receiver:   receiver,
				Fields:     fields,
				ParamsName: paramName,
			}

			api := apis[receiver]
			if api == nil {
				api = &APIServeHTTP{
					Receiver: receiver,
				}
				apis[receiver] = api
			}
			api.Handlers = append(api.Handlers, h)
		}
	}

	for _, api := range apis {
		if err := serveHTTPTpl.Execute(out, api); err != nil {
			log.Fatal(err)
		}
	}
}

func findAndParseValidators(node *ast.File, structName string) []APIField {
	for _, decl := range node.Decls {
		g, ok := decl.(*ast.GenDecl)
		if !ok || g.Tok != token.TYPE {
			continue
		}
		for _, spec := range g.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != structName {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			return parseValidators(st)
		}
	}
	return nil
}

func parseValidators(st *ast.StructType) []APIField {
	var res []APIField

	for _, f := range st.Fields.List {
		if f.Tag == nil || len(f.Names) == 0 {
			continue
		}

		tag := strings.Trim(f.Tag.Value, "`")
		stTag := reflect.StructTag(tag)
		av := stTag.Get("apivalidator")
		if av == "" {
			continue
		}

		v := APIValidator{}
		switch t := f.Type.(type) {
		case *ast.Ident:
			v.Type = t.Name
		default:
			continue
		}

		parts := strings.Split(av, ",")
		for _, p := range parts {
			switch {
			case p == "required":
				v.Required = true
			case strings.HasPrefix(p, "paramname="):
				v.Paramname = strings.TrimPrefix(p, "paramname=")
			case strings.HasPrefix(p, "enum="):
				v.Enum = strings.Split(strings.TrimPrefix(p, "enum="), "|")
			case strings.HasPrefix(p, "default="):
				v.Default = strings.TrimPrefix(p, "default=")
			case strings.HasPrefix(p, "min="):
				n, err := strconv.Atoi(strings.TrimPrefix(p, "min="))
				if err == nil {
					v.Min = &n
				}
			case strings.HasPrefix(p, "max="):
				n, err := strconv.Atoi(strings.TrimPrefix(p, "max="))
				if err == nil {
					v.Max = &n
				}
			}
		}

		res = append(res, APIField{
			Name:      f.Names[0].Name,
			Validator: v,
		})
	}

	return res
}
