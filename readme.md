# In development

Rewritting but works for testing/demostrating a POC via building the binary.

# Description

*EXPECTED OUTPUT*
Application that scans one or a collection of targets (urls), looks for left over git data and either converts the file objects to files locally or displays a list of them

This is a security tool use at own risk

*ACTUAL OUTPUT*
Application that scans a target and converts the git data as represented in index 1-1 to source files locally

# Install and build
First get and build it
`go get github.com/ronnieskansing/gorgit`
`go build`

# Usage
- Test and scrape
`PROJECTNAME -u http://target.tld`
Scraped source is saved at `target.tld/...`


- Use a sock5 proxy
`PROJECTNAME -u http://target.tld -p 127.0.0.1:9150`

...

# Developer notes
~~Pull requests with features, fixes and refactoring are appreciated~~
