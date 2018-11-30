![Gopher robbing git](https://raw.githubusercontent.com/RonnieSkansing/gorgit/master/assets/0.5x/gorgit-logo%400.5x.png)

> credits and thanks to [Paula Sobczak](https://paulajs.dk) for logo based on [Renee French's gophers](http://reneefrench.blogspot.com/)
> licensed under https://creativecommons.org/licenses/by/3.0/



# Description
Most git scrapers require open directory listing in the .git folder or multiple dependencies. This tool requires neither.

Scrapes source code by parsing the `/.git/index` file, downloads the object files referenced and unpacks to the source code.

This is a security reconnaissance tool and is illegal to use without consent from the target is it used upon.

# Install and build
Get it

`go get github.com/ronnieskansing/go-rip-git`

Build it

`go build`

# Usage
### Show files
`go-rip-git -u http://target.tld`

Results in something like
```
c1f3161c27b7fb86615a4916f595473a0a76c774 .env
29c16c3f37ea57569fbf9cc1ce183938a9710aed config/config.json
...
```

## Use a SOCKS5 proxy
`go-rip-git -u http://target.tld -p 127.0.0.1:9150`

## Scrape files
`go-rip-git -u http://target.tld -s true`

**Warning** *This fires up 1 request for each file without any throttle and copies potentially private source code.*

Scraped source is found in `target.tld/...``

# Developer notes / TODO
Pull requests with features, fixes and refactoring are appreciated

Things that come into mind
- Extract contents of .PACK files
- Choose output directory
- Verbosity settings
- Tests
- Accepting a list of targets (from arg and file)
- Throttle control
- Setting of verbosity level

Found a **bug**? Create an issue
