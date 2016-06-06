package main

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	ic "github.com/noxer/intercept"
)

var (
	casBody = cascadia.MustCompile("html body")
)

func main() {
	fmt.Println("Setting up routes...")

	// Create the interceptor
	i, err := ic.DefaultInterceptor("/inject/")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	i.Modifier = append(i.Modifier, func(root *html.Node, baseURL *url.URL) {
		node := casBody.MatchFirst(root)
		if node == nil {
			fmt.Println("There seems to be no body tag, aborting.")
			return
		}
		node.AppendChild(&html.Node{
			Type:     html.ElementNode,
			Data:     "script",
			DataAtom: atom.Script,
			Attr:     []html.Attribute{html.Attribute{Key: "src", Val: "/public/inject.js"}},
		})
	})

	mux := http.NewServeMux()
	mux.Handle("/inject/", http.StripPrefix("/inject/", i))
	mux.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.Dir("public"))))

	fmt.Println("Now serving...")
	fmt.Printf("Error: %s\n", http.ListenAndServe(":8080", mux))
}
