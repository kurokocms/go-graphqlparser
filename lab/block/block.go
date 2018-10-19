package main

import (
	"fmt"

	"github.com/bucketd/go-graphqlparser/language"
	"github.com/bucketd/go-graphqlparser/token"
	"github.com/davecgh/go-spew/spew"
)

const query = "query {\r\n  sendEmail(message: \"\"\"\r\n    Hello GraphQL,\r\n\r\n    This is weird\r\n  \"\"\")\r\n}"

func main() {
	lxr := language.NewLexer([]byte(query))

	fmt.Println(query)

	for {
		tok := lxr.Scan()
		if tok.Type == token.EOF {
			break
		}

		spew.Dump(tok)
	}
}
