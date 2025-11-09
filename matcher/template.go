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

type templateMatcher struct {
	texts []string
	tmpls []*template.Template
}

func (tm *templateMatcher) Ignore() bool {
	return len(tm.tmpls) < 1
}

func newTemplateMatcher(texts []string) (*templateMatcher, error) {
	tmpls := make([]*template.Template, len(texts))

	globCache := &sync.Map{}
	regexpCache := &sync.Map{}

	funcmap := template.FuncMap{
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
		tmpl, err := template.New(text).Funcs(funcmap).Parse(text)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template text '%v': %w", text, err)
		}

		tmpls[i] = tmpl
	}

	return &templateMatcher{
		texts: texts,
		tmpls: tmpls,
	}, nil
}

func (tm *templateMatcher) Matches(file *walk.File) (bool, error) {
	for _, tmpl := range tm.tmpls {
		var buf bytes.Buffer

		err := tmpl.Execute(&buf, file)
		if err != nil {
			return false, fmt.Errorf("failed to execute template in the context of %s: %w", file, err)
		}

		match, err := strconv.ParseBool(buf.String())
		if err != nil {
			return false, fmt.Errorf("error parsing template result as boolean: %w", err)
		}

		if match {
			return match, nil
		}
	}

	return false, nil
}

type TemplateInclusionMatcher struct {
	*inclusionMatcher
	templateMatcher
}

//nolint:ireturn
func NewTemplateInclusionMatcher(texts []string) (Matcher, error) {
	tm, err := newTemplateMatcher(texts)
	if err != nil {
		return nil, err
	}

	return &TemplateInclusionMatcher{templateMatcher: *tm}, nil
}

type TemplateExclusionMatcher struct {
	*exclusionMatcher
	templateMatcher
}

//nolint:ireturn
func NewTemplateExclusionMatcher(texts []string) (Matcher, error) {
	tm, err := newTemplateMatcher(texts)
	if err != nil {
		return nil, err
	}

	return &TemplateExclusionMatcher{templateMatcher: *tm}, nil
}
