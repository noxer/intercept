package intercept

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	notFound = "<!DOCTYPE html><html><head><title>Not found</title></head><body><h1>404 not found</h1>Sorry, we could not find the requested page!</body></html>"
)

// Intercepter enables us to inject tags into HTML pages while they are loaded
type Intercepter struct {
	Modifier []func(*html.Node, *url.URL)
	BaseURL  *url.URL
}

// DefaultIntercepter creates a new intercepter with a DefaultModifier
func DefaultIntercepter(base string) (*Intercepter, error) {
	// Prepare the URL for use as the prefix
	baseURL, err := normalizeURL(base)
	if err != nil {
		return nil, err
	}
	// Create the intercepter
	return &Intercepter{
		Modifier: []func(*html.Node, *url.URL){DefaultModifier(baseURL)},
		BaseURL:  baseURL,
	}, nil
}

// DefaultModifier creates a default DOM modifier that prefixes all links with baseURL
func DefaultModifier(baseURL *url.URL) func(*html.Node, *url.URL) {
	return func(root *html.Node, pageURL *url.URL) {
		modifyDOM(root, pageURL, baseURL)
	}
}

func modifyDOM(node *html.Node, pageURL, baseURL *url.URL) {
	// Traverse all siblings
	for ; node != nil; node = node.NextSibling {
		// Iterate over all attributes
		for i, a := range node.Attr {
			switch {
			case node.DataAtom == atom.A && a.Key == "href", // A link href
				node.DataAtom == atom.Form && a.Key == "action": // A form action
				a.Val = localURL(a.Val, pageURL, baseURL)
				node.Attr[i] = a

			case a.Key == "src", // A href
				a.Key == "href",     // A src
				a.Key == "data-src": // AngularJS
				a.Val = remoteURL(a.Val, pageURL)
				node.Attr[i] = a
			}
		}

		// Traverse the children
		modifyDOM(node.FirstChild, pageURL, baseURL)
	}
}

func localURL(url string, pageURL, baseURL *url.URL) string {
	switch {
	case strings.HasPrefix(url, "//"): // URL with same schema as page
		return fmt.Sprintf("%s%s:%s", baseURL, pageURL.Scheme, url)

	case strings.HasPrefix(url, "/"): // URL with same host as page
		return fmt.Sprintf("%s%s%s", baseURL, hostURL(pageURL), url)

	case strings.HasPrefix(url, "http://"),
		strings.HasPrefix(url, "https://"):
		return fmt.Sprintf("%s%s", baseURL, url) // A normal link, just prefix it

	case strings.HasPrefix(url, "mailto:"): // It's an email address, don't change it
		return url
	}
	return fmt.Sprintf("%s%s%s", baseURL, hostURL(pageURL), path.Join(path.Dir(pageURL.RawPath), url)) // Join the relative path
}

func remoteURL(url string, pageURL *url.URL) string {
	switch {
	case strings.HasPrefix(url, "//"): // URL with same schema as page
		return fmt.Sprintf("%s:%s", pageURL.Scheme, url)

	case strings.HasPrefix(url, "/"): // URL with the same host as page
		return fmt.Sprintf("%s%s", hostURL(pageURL), url)

	case strings.HasPrefix(url, "http://"),
		strings.HasPrefix(url, "https://"):
		// Nothing to do, link is already external
		return url

	case strings.HasPrefix(url, "mailto:"): // It's an email address, don't change it
		return url
	}
	return fmt.Sprintf("%s%s", hostURL(pageURL), path.Join(path.Dir(pageURL.RawPath), url)) // Join the relative path
}

func hostURL(u *url.URL) string {
	buf := &bytes.Buffer{}
	// Append the schema
	if u.Scheme == "" {
		buf.WriteString("http://")
	} else {
		buf.WriteString(u.Scheme)
		buf.WriteString("://")
	}
	// Append the user
	if u.User != nil {
		buf.WriteString(u.User.String())
		buf.WriteByte('@')
	}
	// Append the host
	buf.WriteString(u.Host)
	return buf.String()
}

func normalizeURL(baseURL string) (*url.URL, error) {
	// Parse the URL
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	// Make sure we have got a scheme
	if u.Scheme != "http" && u.Scheme != "https" && u.Host != "" {
		u.Scheme = "http"
	}
	// Remove query parameters and the fragment
	u.RawQuery = ""
	u.Fragment = ""
	// Make sure there is a tailing slash
	if u.Path[len(u.Path)-1] != '/' {
		u.Path += "/"
	}
	return u, nil
}

// ServeHTTP implements the http.Handler interface
func (i *Intercepter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Create the request to the real server
	u := r.URL.String()
	u = strings.TrimLeft(u, "/")

	// Fix the http:/ issue
	if strings.HasPrefix(u, "http:") {
		u = "http://" + strings.TrimLeft(strings.TrimPrefix(u, "http:"), "/")
	} else if strings.HasPrefix(u, "http:") {
		u = "https://" + strings.TrimLeft(strings.TrimPrefix(u, "https:"), "/")
	}

	m := r.Method
	if m == "" {
		m = "GET"
	}

	newReq, err := http.NewRequest(m, u, r.Body)
	if err != nil {
		respond404(w)
		return
	}

	// Set basic auth if provided
	if user, pass, ok := r.BasicAuth(); ok {
		newReq.SetBasicAuth(user, pass)
	}

	// Perform the request
	resp, err := http.DefaultClient.Do(newReq)
	if err != nil {
		respond404(w)
		return
	}
	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		respond404(w)
		return
	}

	// Try to parse the result as html
	doc, err := html.Parse(bytes.NewReader(b))
	if err != nil {
		w.WriteHeader(resp.StatusCode)
		w.Write(b)
		return
	}

	// Manipulate DOM

	// Replace all href and src
	pageURL, err := resp.Location()
	if err != nil {
		pageURL = newReq.URL
	}
	for _, modify := range i.Modifier {
		modify(doc, pageURL)
	}

	// Write the answer to the client
	w.WriteHeader(resp.StatusCode)
	if err = html.Render(w, doc); err != nil {
	}
}

func respond404(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprint(w, notFound)
}
