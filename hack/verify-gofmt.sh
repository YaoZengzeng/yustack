#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

find_files() {
	find . -not \( \
		\( \
		  -wholename '*/vendor/*' \
		\) -prune \
	  \) -name '*.go'
}

GOFMT="gofmt -w"

