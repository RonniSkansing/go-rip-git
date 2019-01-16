build-release:
	rm -rf bin/*
	GOOS=linux GOARCH=386 go build -o bin/go-rip-git
	GOOS=windows GOARCH=386 go build -o bin/go-rip-git.exe
