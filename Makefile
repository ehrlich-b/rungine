# Rungine Makefile
# Ubuntu 24.04 requires webkit2_41 tag

WAILS_TAGS := webkit2_41

.PHONY: dev build clean

dev:
	wails dev -tags $(WAILS_TAGS)

build:
	wails build -tags $(WAILS_TAGS)

clean:
	rm -rf build/bin
