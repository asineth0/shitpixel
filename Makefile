build:
	@rm -rf dist && mkdir -p dist
	@LDFLAGS='-s -w' go build -o dist
	@cp message.json dist/