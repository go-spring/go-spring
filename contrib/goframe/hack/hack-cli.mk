
# Install/Update to the latest CLI tool.
.PHONY: cli
cli:
	@set -e; \
    echo "go install github.com/gogf/gf/cmd/gf/v2@latest"; \
	go install github.com/gogf/gf/cmd/gf/v2@latest; \
	echo "GoFame CLI installed successfully!"


# Check and install CLI tool.
.PHONY: cli.install
cli.install:
	@set -e; \
	gf -v > /dev/null 2>&1 || if [[ "$?" -ne "0" ]]; then \
  		echo "GoFame CLI is not installed, start proceeding auto installation..."; \
		make cli; \
	fi;