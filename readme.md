# Description

*EXPECTED OUTPUT*
Application that scans one or a collection of targets (urls), looks for left over git data and either converts the file objects to files locally or displays a list of them

This is a security tool use at own risk or whatever

*ACTUAL OUTPUT*
Application that scans a target and converts the git data to source files locally

# How to use
First get and build it
`go get github.com/ronnieskansing/PROJECTNAME`
`go build`

Use it
`PROJECTNAME -u http://target.tld`

Use a sock5 proxy
`PROJECTNAME -u http://target.tld -p 127.0.0.1:9150`

...

// TODO tefactor 3x, then segregate packages
// TODO improve output format
// TODO verbosity flag
// TODO wishlist: help command, banner, throttle, dont scrape, just show index data, replace proxy package with native
