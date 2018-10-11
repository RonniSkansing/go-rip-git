![Gopher robbing git](https://raw.githubusercontent.com/RonnieSkansing/gorgit/master/assets/0.5x/gorgit-logo%400.5x.png)

*Cute gopher Logo credits goes to [Paula Sobczak](https://paulajs.dk) based on [Renee French's gophers](http://reneefrench.blogspot.com/). 
Logo is licensed under https://creativecommons.org/licenses/by/3.0/*



# Description
When deploying it is important to remove or cut off access to /.git folder

This program is used to extract information and pull the remote files locally.

Use responsibly - Do no use on targets without prior permission. Also take when scraping the remote files, depending on the size of the repo, you might fire off more requests faster then you expect.

Use at your own own risk.


# Install and build
Get it

`go get github.com/ronnieskansing/gorgit`

Build it

`go build`

# Usage
### Show files
`gorgit -u http://target.tld`

Results in something like
```
c1f3161c27b7fb86615a4916f595473a0a76c774 .env
29c16c3f37ea57569fbf9cc1ce183938a9710aed config/config.json
...
```

## Use a SOCKS5 proxy
`gorgit -u http://target.tld -p 127.0.0.1:9150`

## Scrape files
`gorgit -u http://target.tld -s true`

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
