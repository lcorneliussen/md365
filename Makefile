PREFIX ?= $(HOME)/.local

install:
	install -Dm755 md365 $(PREFIX)/bin/md365

uninstall:
	rm -f $(PREFIX)/bin/md365

.PHONY: install uninstall
