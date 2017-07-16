# In development

Rewritting but works for testing/demostrating a POC via building the binary.

# Description

*EXPECTED OUTPUT*
Application that scans one or a collection of targets (urls), looks for left over git data and either converts the file objects to files locally or displays a list of them

This is a security tool use at own risk

*ACTUAL OUTPUT*
Application that scans a target and either displays the file content of git index or scrapes it down locally

# Install and build
First get and build it
`go get github.com/ronnieskansing/gorgit`
`go build`

# Usage
- Test and show files
`gorgit -u http://target.tld`

Results in something like
```
c1f3161c27b7fb86615a4916f595473a0a76c774 .env
29c16c3f37ea57569fbf9cc1ce183938a9710aed config/config.json
...
```

- Use a sock5 proxy
`gorgit -u http://target.tld -p 127.0.0.1:9150`

- Scrape files
**Warning** *This fires up 1 request for each file without any throttle and copies potentially private source code.*
`gorgit -u http://target.tld -s true`

Scraped source is found in `target.tld/...``

# Developer notes
~~Pull requests with features, fixes and refactoring are appreciated~~

Found a **bug**? Create a issue
