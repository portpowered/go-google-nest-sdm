package main

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestBehaviorMethodRequiresActualReceiverDeclaration(t *testing.T) {
	t.Parallel()

	for name, source := range map[string]string{
		"declared behavior": `package behavior; type Session struct{}
func (session *Session) exchange() {}
func (session *Session) Complete(){session.exchange()}`,
		"shadowed receiver": `package behavior; type Session struct{}
func (session *Session) exchange() {}
func (session *Session) Complete(){if true {session:=&Session{};session.exchange()}}`,
		"unresolved receiver method": `package behavior; type Session interface{exchange()}
func Complete(session Session){session.exchange()}`,
		"outbound behavior": `package behavior; import "net/http"; type Session struct{}
func (session *Session) exchange(){_,_=http.Get("https://example.invalid")}
func (session *Session) Complete(){session.exchange()}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			file, err := parser.ParseFile(token.NewFileSet(), "behavior.go", source, 0)
			if err != nil {
				t.Fatal(err)
			}

			err = auditFile(file, "pkg/dependencies/behavior/session.go")
			if (err == nil) != (name == "declared behavior") {
				t.Fatalf("method binding acceptance differs: %v", err)
			}
		})
	}
}
