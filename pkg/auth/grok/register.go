package grok

import "charm.land/catwalk/pkg/catwalk"

func init() {
	a := New()
	catwalk.RegisterAuth(a)
}
