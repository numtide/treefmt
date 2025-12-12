package matcher

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"sync"
	"text/template"

	"github.com/gobwas/glob"
	"github.com/numtide/treefmt/v2/walk"
)

func IncludeTemplates(texts []string) (MatchFn, error) {
	if len(texts) == 0 {
		return noOp, nil
	}

	templates := make([]*template.Template, len(texts))

	globCache := &sync.Map{}
	regexpCache := &sync.Map{}

	funcMap := template.FuncMap{
		"fnmatch": func(pattern string, s string) bool {
			cached, ok := globCache.Load(pattern)

			if ok {
				if entry, ok := cached.(glob.Glob); ok {
					return entry.Match(s)
				}
			}

			g := glob.MustCompile(pattern)

			globCache.Store(pattern, g)

			return g.Match(s)
		},
		"rematch": func(pattern string, s string) bool {
			cached, ok := regexpCache.Load(pattern)

			if ok {
				if entry, ok := cached.(regexp.Regexp); ok {
					return entry.MatchString(s)
				}
			}

			re := regexp.MustCompile(pattern)

			regexpCache.Store(pattern, re)

			return re.MatchString(s)
		},
	}

	for i, text := range texts {
		tmpl, err := template.New(text).Funcs(funcMap).Parse(text)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template text '%v': %w", text, err)
		}

		templates[i] = tmpl
	}

	return func(file *walk.File) (Result, error) {
		for _, tmpl := range templates {
			var buf bytes.Buffer

			err := tmpl.Execute(&buf, file)
			if err != nil {
				return Error, fmt.Errorf("failed to execute template in the context of %s: %w", file, err)
			}

			match, err := strconv.ParseBool(buf.String())
			if err != nil {
				return Error, fmt.Errorf("error parsing template result as boolean: %w", err)
			}

			if match {
				return Wanted, nil
			}
		}

		return Indifferent, nil
	}, nil
}

func ExcludeTemplates(texts []string) (MatchFn, error) {
	includeFn, err := IncludeTemplates(texts)

	return invert(includeFn), err
}
