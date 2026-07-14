LOCAL_TOOLS_PATH := $(CURDIR)/.tools/bin
LOCAL_GO_PATH := /usr/local/go/bin

.PHONY: dev build clean

dev:
	@PATH="$(LOCAL_TOOLS_PATH):$(LOCAL_GO_PATH):$$PATH"; \
	if command -v wails >/dev/null 2>&1; then \
		echo "[dev] wails dev"; \
		wails dev -tags webkit2_41; \
	else \
		echo "[dev] fallback to Vite"; \
		cd frontend && npm run dev; \
	fi

build:
	@PATH="$(LOCAL_TOOLS_PATH):$(LOCAL_GO_PATH):$$PATH" \
		wails build -platform linux/amd64 -clean -tags webkit2_41

clean:
	rm -rf build/bin
