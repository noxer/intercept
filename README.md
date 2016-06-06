# intercept
Modify HTML pages on the fly.

## Problem
Sometimes I need to change webpages while requesting them for the user, in my special case I wanted to inject a script tag that loads a DOM selector library into the requested page.

## Solution
This library offers an "Interceptor" that implements the http.Handler interface. Any request made to the Interceptor is then relayed to the server specified in the request and the returned page is modified before being sent back to the client.

e.g.
The page you want to modify is `https://golang.org/dl/`. You redirect the user to `https://example.com/intercept/https:/golang.org/dl/`. The Interceptor establishes a connection to `https://golang.org/dl/` and downloads the page. It modifies it and returns it to the user. If you use the DefaultModifier, it rewrites all (static) `<a>` links and `<form>` actions to use `https://example.com/intercept`.
