package silon

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestDeprecatedOpsCarryDeprecatedMarkers verifies that every deprecated
// operation carries Go's standard deprecation marker — a doc-comment
// paragraph starting with "Deprecated:" — which is what godoc, gopls and
// staticcheck surface to callers (Go's equivalent of Python's
// DeprecationWarning).
func TestDeprecatedOpsCarryDeprecatedMarkers(t *testing.T) {
	cases := []struct {
		file     string
		receiver string
		method   string
	}{
		{"push.go", "PushService", "ListNotifications"},
		{"push.go", "PushService", "SubscribeWeb"},
		{"account.go", "AuthService", "Login"},
		{"bulk.go", "BulkService", "Send"},
		{"crm.go", "ClientsService", "List"},
		{"crm.go", "ClientGroupsService", "List"},
	}

	fset := token.NewFileSet()
	parsed := map[string]*ast.File{}
	for _, tc := range cases {
		if _, ok := parsed[tc.file]; ok {
			continue
		}
		f, err := parser.ParseFile(fset, tc.file, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", tc.file, err)
		}
		parsed[tc.file] = f
	}

	for _, tc := range cases {
		doc, found := methodDoc(parsed[tc.file], tc.receiver, tc.method)
		if !found {
			t.Errorf("%s: method (*%s).%s not found", tc.file, tc.receiver, tc.method)
			continue
		}
		if !hasDeprecatedParagraph(doc) {
			t.Errorf("(*%s).%s lacks a \"Deprecated:\" doc-comment paragraph", tc.receiver, tc.method)
		}
	}
}

// methodDoc returns the doc comment text of the named method on the
// named receiver type.
func methodDoc(f *ast.File, receiver, method string) (string, bool) {
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name != method || fd.Recv == nil || len(fd.Recv.List) != 1 {
			continue
		}
		recvType, ok := fd.Recv.List[0].Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		ident, ok := recvType.X.(*ast.Ident)
		if !ok || ident.Name != receiver {
			continue
		}
		if fd.Doc == nil {
			return "", true
		}
		return fd.Doc.Text(), true
	}
	return "", false
}

// hasDeprecatedParagraph reports whether a doc comment contains a
// paragraph starting with the standard "Deprecated:" marker.
func hasDeprecatedParagraph(doc string) bool {
	for _, para := range strings.Split(doc, "\n\n") {
		if strings.HasPrefix(strings.TrimSpace(para), "Deprecated:") {
			return true
		}
	}
	return false
}
